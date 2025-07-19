// cmd/order-service/main.go
package main

import (
	"context"
	"log"
	"net/http"
	"nexus/internal/pkg/bootstrap"
	"nexus/internal/pkg/httpclient"
	"nexus/internal/pkg/mq"
	"nexus/internal/pkg/nacos"
	"nexus/internal/service/order/application"
	"nexus/internal/service/order/infrastructure"
	"nexus/internal/service/order/infrastructure/adapter"
	"nexus/internal/service/order/interfaces"
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
}

// assembleDependencies 负责创建所有依赖项
func assembleDependencies(nacosClient *nacos.Client) (*Dependencies, error) {
	brokers := strings.Split(bootstrap.GetCurrentConfig().Infra.Kafka.Brokers, ",")

	// 所有依赖都在这里统一创建
	deps := &Dependencies{
		NotificationWriter:  mq.NewKafkaWriter(brokers, notificationTopic),
		DelayWriter:         mq.NewKafkaWriter(brokers, delayTopics),
		OrderCreationWriter: mq.NewKafkaWriter(brokers, orderCreationTopic),
		OrderCreationReader: mq.NewKafkaReader(brokers, orderCreationTopic, orderCreationConsumerGroupID),
		TimeoutCheckReader:  mq.NewKafkaReader(brokers, orderTimeoutCheckTopic, timeoutCheckConsumerGroupID),
		OrderRepo:           infrastructure.NewMysqlRepository(),
	}

	// 组装 Application Service
	tracer := otel.Tracer(serviceName)
	httpClient := httpclient.NewClient(tracer, nacosClient) // Nacos client will be available later if needed
	fraudAdapter := adapter.NewFraudHTTPAdapter(httpClient)
	inventoryAdapter := adapter.NewInventoryHTTPAdapter(httpClient)
	pricingAdapter := adapter.NewPricingHTTPAdapter(httpClient)
	schedulerAdapter := adapter.NewSchedulerKafkaAdapter(deps.DelayWriter)
	notificationAdapter := adapter.NewNotificationKafkaAdapter(deps.NotificationWriter)

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
	orderConsumer := interfaces.NewOrderConsumerAdapter(deps.OrderCreationReader, deps.AppService)
	app.AddTask(orderConsumer.Start, func(ctx context.Context) error {
		orderConsumer.Stop()
		return nil
	})

	timeoutConsumer := interfaces.NewOrderTimeOutConsumerAdapter(deps.TimeoutCheckReader, deps.AppService)
	app.AddTask(timeoutConsumer.Start, func(ctx context.Context) error {
		timeoutConsumer.Stop()
		return nil
	})

	// 3. 注册其他需要关闭的资源
	app.AddTask(nil, func(ctx context.Context) error {
		log.Println("Closing kafka writers...")
		deps.NotificationWriter.Close()
		deps.DelayWriter.Close()
		deps.OrderCreationWriter.Close()
		return nil
	})

	return nil
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

	// 创建并运行应用
	app, err := bootstrap.NewApplication(appInfo)
	if err != nil {
		log.Fatalf("❌ Failed to create application: %v", err)
	}

	if err := app.Run(); err != nil {
		log.Fatalf("❌ Application run failed: %v", err)
	}
}
