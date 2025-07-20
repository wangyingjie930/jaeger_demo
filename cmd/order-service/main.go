// cmd/order-service/main.go
package main

import (
	"context"
	"net/http"
	"nexus/internal/pkg/bootstrap"
	"nexus/internal/pkg/httpclient"
	"nexus/internal/pkg/logger"
	"nexus/internal/pkg/mq"
	"nexus/internal/pkg/nacos"
	"nexus/internal/pkg/redis"
	"nexus/internal/service/order/application"
	"nexus/internal/service/order/infrastructure"
	"nexus/internal/service/order/infrastructure/adapter"
	"nexus/internal/service/order/interfaces"
	"strconv"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
)

const (
	serviceName                  = "order-service"
	notificationTopic            = "notifications"
	orderCreationTopic           = "order-creation-topic"
	orderCreationConsumerGroupID = "order-creation-consumer-group"
	orderTimeoutCheckTopic       = "order-timeout-check-topic"
	delayTopics                  = "delay_topic_5s"
	timeoutCheckConsumerGroupID  = "timeout-check-consumer-group"
)

// Dependencies 结构体包含了 order-service 所需的所有依赖项
type Dependencies struct {
	NotificationWriter  *kafka.Writer
	DelayWriter         *kafka.Writer
	OrderCreationWriter *kafka.Writer
	OrderCreationReader *kafka.Reader
	TimeoutCheckReader  *kafka.Reader
	OrderRepo           *infrastructure.MysqlRepository
	AppService          *application.OrderApplicationService
	RedisClient         *redis.Client

	FailureHandler *mq.FailureHandler
}

// assembleDependencies 负责创建所有依赖项
func assembleDependencies(nacosClient *nacos.Client) (*Dependencies, error) {
	brokers := strings.Split(bootstrap.GetCurrentConfig().Infra.Kafka.Brokers, ",")

	// 组装 Application Service
	tracer := otel.Tracer(serviceName)

	// 所有依赖都在这里统一创建
	deps := &Dependencies{
		NotificationWriter:  mq.NewKafkaWriter(brokers, notificationTopic),
		DelayWriter:         mq.NewKafkaWriter(brokers, delayTopics),
		OrderCreationWriter: mq.NewKafkaWriter(brokers, orderCreationTopic),
		OrderCreationReader: mq.NewKafkaReader(brokers, orderCreationTopic, orderCreationConsumerGroupID),
		TimeoutCheckReader:  mq.NewKafkaReader(brokers, orderTimeoutCheckTopic, timeoutCheckConsumerGroupID),
		OrderRepo:           infrastructure.NewMysqlRepository(),
		FailureHandler: mq.NewFailureHandler(brokers, mq.ResilienceConfig{
			Enabled:             bootstrap.GetCurrentConfig().App.Resilience.Consumers["orderCreation"].Enabled,
			RetryDelays:         bootstrap.GetCurrentConfig().App.Resilience.Consumers["orderCreation"].RetryDelays,
			RetryTopicTemplate:  bootstrap.GetCurrentConfig().App.Resilience.Consumers["orderCreation"].RetryTopicTemplate,
			DltTopicTemplate:    bootstrap.GetCurrentConfig().App.Resilience.Consumers["orderCreation"].DltTopicTemplate,
			RetryableExceptions: bootstrap.GetCurrentConfig().App.Resilience.Consumers["orderCreation"].RetryableExceptions,
		}, tracer),
	}

	httpClient := httpclient.NewClient(tracer, nacosClient) // Nacos client will be available later if needed
	redisCilent, _ := redis.NewClient(bootstrap.GetCurrentConfig().Infra.Redis.Addrs)
	fraudAdapter := adapter.NewFraudHTTPAdapter(httpClient)
	inventoryAdapter := adapter.NewInventoryHTTPAdapter(httpClient)
	pricingAdapter := adapter.NewPricingHTTPAdapter(httpClient)
	schedulerAdapter := adapter.NewSchedulerKafkaAdapter(deps.DelayWriter)
	notificationAdapter := adapter.NewNotificationKafkaAdapter(deps.NotificationWriter)
	seckillAdapter, _ := adapter.NewSeckillRedisAdapter(redisCilent)

	deps.AppService = application.NewOrderApplicationService(
		deps.OrderRepo,
		5*time.Second,
		tracer,
		infrastructure.NewOrderProducerAdapter(deps.OrderCreationWriter),
		fraudAdapter,
		inventoryAdapter,
		pricingAdapter,
		pricingAdapter,
		schedulerAdapter,
		notificationAdapter,
		seckillAdapter,
	)

	return deps, nil
}

// registerService 负责注册所有服务和后台任务
func registerService(app *bootstrap.Application, deps *Dependencies) error {
	// 1. 注册 HTTP 服务
	httpHandler := interfaces.NewOrderHandler(deps.AppService)
	mux := http.NewServeMux()
	httpHandler.RegisterRoutes(mux)
	app.AddServer(mux, 8081)

	// 2. 注册 Kafka 消费者作为后台任务
	orderConsumer := interfaces.NewOrderConsumerAdapter(deps.OrderCreationReader, deps.AppService, deps.FailureHandler)
	app.AddTask(orderConsumer.Start, func(ctx context.Context) error {
		orderConsumer.Stop(ctx)
		return nil
	})

	timeoutConsumer := interfaces.NewOrderTimeOutConsumerAdapter(deps.TimeoutCheckReader, deps.AppService)
	app.AddTask(timeoutConsumer.Start, func(ctx context.Context) error {
		timeoutConsumer.Stop(ctx)
		return nil
	})

	// 3. 注册其他需要关闭的资源
	app.AddTask(nil, func(ctx context.Context) error {
		logger.Ctx(ctx).Println("Closing kafka writers...")
		deps.NotificationWriter.Close()
		deps.DelayWriter.Close()
		deps.OrderCreationWriter.Close()
		return nil
	})

	registerRetryAndDltConsumers(app, deps)

	return nil
}

// ✨ 新建函数：注册所有重试和DLT消费者
func registerRetryAndDltConsumers(app *bootstrap.Application, deps *Dependencies) {
	consumerConfig := bootstrap.GetCurrentConfig().App.Resilience.Consumers["orderCreation"]
	if !consumerConfig.Enabled {
		return
	}
	brokers := strings.Split(bootstrap.GetCurrentConfig().Infra.Kafka.Brokers, ",")

	// 注册重试Topic的消费者
	for _, delay := range consumerConfig.RetryDelays {
		retryTopic := strings.NewReplacer(
			"{topic}", orderCreationTopic,
			"{delaySec}", strconv.Itoa(delay),
		).Replace(consumerConfig.RetryTopicTemplate)

		retryReader := mq.NewKafkaReader(brokers, retryTopic, orderCreationConsumerGroupID+"-retry")
		// 重试消费者也需要注入 FailureHandler，以便在失败时能进入下一个重试环节或DLT
		retryConsumer := interfaces.NewOrderConsumerAdapter(retryReader, deps.AppService, deps.FailureHandler)
		// ✨ 为重试消费者添加延迟逻辑
		retryConsumer.SetDelay(time.Duration(delay) * time.Second)

		app.AddTask(retryConsumer.Start, func(ctx context.Context) error {
			retryConsumer.Stop(ctx)
			return nil
		})
	}

	// 注册DLT消费者
	dltTopic := strings.NewReplacer("{topic}", orderCreationTopic).Replace(consumerConfig.DltTopicTemplate)
	dltReader := mq.NewKafkaReader(brokers, dltTopic, orderCreationConsumerGroupID+"-dlt")
	dltConsumer := interfaces.NewDltConsumerAdapter(dltReader) // 使用新的DLT专用消费者

	app.AddTask(dltConsumer.Start, func(ctx context.Context) error {
		dltConsumer.Stop(ctx)
		return nil
	})

	logger.Logger.Info().Msg("✅ Retry and DLT consumers registered.")
}

func main() {
	// 描述应用
	appInfo := bootstrap.AppInfoV2[*Dependencies]{
		ServiceName: serviceName,
		Assemble: func(appCtx bootstrap.AppContext) (*Dependencies, error) {
			// 将匿名函数替换为具名函数，更清晰
			return assembleDependencies(appCtx.NamingClient)
		},
		Register: func(app *bootstrap.Application, deps *Dependencies) error {
			// 将匿名函数替换为具名函数
			return registerService(app, deps)
		},
	}

	ctx := context.Background()
	// 创建并运行应用
	app, err := bootstrap.NewApplication(appInfo)
	if err != nil {
		logger.Ctx(ctx).Fatal().Msgf("❌ Failed to create application: %v", err)
	}

	if err := app.Run(); err != nil {
		logger.Ctx(ctx).Fatal().Msgf("❌ Application run failed: %v", err)
	}
}
