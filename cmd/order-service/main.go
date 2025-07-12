// cmd/order-service/main.go
package main

import (
	"context"
	"encoding/json"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/trace"
	"jaeger-demo/internal/pkg/bootstrap"
	"jaeger-demo/internal/pkg/httpclient"
	"jaeger-demo/internal/pkg/mq"
	"jaeger-demo/internal/pkg/redis"
	orderSvc "jaeger-demo/internal/service/order"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

const (
	serviceName                  = "order-service"
	notificationTopic            = "notifications"
	orderProcessingTimeout       = 30 * time.Second // 单个订单处理流程的超时上限
	orderCreationTopic           = "order-creation-topic"
	orderCreationConsumerGroupID = "order-creation-consumer-group"

	// ✨ 新增: 定义超时检查的Topic和相关配置
	orderTimeoutCheckTopic      = "order-timeout-check-topic" // 用于接收到期检查任务的Topic
	timeoutCheckConsumerGroupID = "timeout-check-consumer-group"
	delayTopics                 = "delay_topic_5s"
)

// 从环境变量或配置中读取所有下游服务的URL
var (
	jaegerEndpoint = getEnv("JAEGER_ENDPOINT", "http://localhost:14268/api/traces")
	kafkaBrokers   = getEnv("KAFKA_BROKERS", "localhost:9092")
	redisAddrs     = getEnv("REDIS_ADDRS", "localhost:6379,localhost:6380,localhost:6381")
)

// main 函数是应用的"组装根" (Composition Root)
// 它的核心职责是：创建并组装所有依赖项，然后启动应用。
func main() {
	redisClient, err := redis.NewClient(redisAddrs)
	if err != nil {
		log.Fatalf("failed to initialize redis client: %v", err)
	}

	// ✨ [核心改造] 初始化业务 Service
	seckillService := orderSvc.NewSeckillService(redisClient)
	// (可选, 用于测试) 准备一个秒杀商品
	err = seckillService.PrepareSeckillProduct(context.Background(), "product_123", 100)
	if err != nil {
		log.Printf("WARN: could not prepare seckill product for testing: %v", err)
	}

	// 2. 构建责任链 (将依赖项预先绑定)
	// 这是一个最佳实践，将依赖注入和业务链的构建分离
	orderChain := buildOrderProcessingChain(seckillService)

	// 这个Writer专门用于向“通知主题”发送消息
	notificationKafkaWriter := mq.NewKafkaWriter(strings.Split(kafkaBrokers, ","), notificationTopic)
	defer notificationKafkaWriter.Close()

	kafkaDelayWriters := make(map[string]*kafka.Writer)
	for _, delayTopic := range strings.Split(delayTopics, ",") {
		kafkaDelayWriters[delayTopic] = mq.NewKafkaWriter(strings.Split(kafkaBrokers, ","), delayTopic)
		defer kafkaDelayWriters[delayTopic].Close()
	}

	bootstrap.StartService(bootstrap.AppInfo{
		ServiceName: serviceName,
		Port:        8081,
		RegisterHandlers: func(appCtx bootstrap.AppCtx) {
			tracer := otel.Tracer(serviceName)
			httpClient := httpclient.NewClient(tracer, appCtx.Nacos)

			appCtx.Mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
			appCtx.Mux.Handle("/metrics", promhttp.Handler())

			go func() {
				// ✨ 核心改造: 使用 WaitGroup 管理两个消费者 Goroutine
				var wg sync.WaitGroup
				wg.Add(2)

				// Goroutine 1: 消费订单创建消息 (基本不变)
				go func() {
					defer wg.Done()
					orderCreationReader := mq.NewKafkaReader(
						strings.Split(kafkaBrokers, ","),
						orderCreationTopic,
						orderCreationConsumerGroupID,
					)
					defer orderCreationReader.Close()
					log.Printf("✅ Order Creation Consumer started. Listening to topic '%s'...", orderCreationTopic)
					for {
						msg, err := orderCreationReader.ReadMessage(context.Background())
						if err != nil {
							log.Printf("ERROR: could not read message from '%s': %v. Retrying...", orderCreationTopic, err)
							time.Sleep(5 * time.Second)
							continue
						}
						go processOrderMessage(msg, httpClient, notificationKafkaWriter, kafkaDelayWriters, orderChain, tracer)
					}
				}()

				// Goroutine 2: ✨ 新增 - 消费订单超时检查消息
				go func() {
					defer wg.Done()
					timeoutCheckReader := mq.NewKafkaReader(
						strings.Split(kafkaBrokers, ","),
						orderTimeoutCheckTopic,
						timeoutCheckConsumerGroupID,
					)
					defer timeoutCheckReader.Close()
					log.Printf("✅ Order Timeout Consumer started. Listening to topic '%s'...", orderTimeoutCheckTopic)
					for {
						msg, err := timeoutCheckReader.ReadMessage(context.Background())
						if err != nil {
							log.Printf("ERROR: could not read message from '%s': %v. Retrying...", orderTimeoutCheckTopic, err)
							time.Sleep(5 * time.Second)
							continue
						}
						go orderSvc.ProcessTimeoutCheckMessage(msg, httpClient)
					}
				}()

				wg.Wait()
			}()
		},
	})
}

// buildOrderProcessingChain 负责构建和连接责任链中的所有处理器
func buildOrderProcessingChain(seckillSvc *orderSvc.SeckillService) orderSvc.Handler {
	orderHandler := new(orderSvc.TransactionHandler)
	orderHandler.SetNext(orderSvc.NewSeckillHandler(seckillSvc)). // 注入 SeckillService
									SetNext(new(orderSvc.FraudCheckHandler)).
									SetNext(new(orderSvc.InventoryReserveHandler)).
									SetNext(new(orderSvc.PriceHandler)).
									SetNext(new(orderSvc.ProcessHandler)).
									SetNext(new(orderSvc.NotificationHandler))

	return orderHandler
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// processOrderMessage 是每个Kafka消息的处理核心，实现了完整的业务流程
func processOrderMessage(
	msg kafka.Message,
	httpClient *httpclient.Client,
	kafkaNotifyWriter *kafka.Writer,
	kafkaDelayWriters map[string]*kafka.Writer,
	chain orderSvc.Handler,
	tracer trace.Tracer,
) {
	// 1. 解析消息体
	var event orderSvc.OrderCreationEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		log.Printf("ERROR: Failed to unmarshal event: %v. Message moved to DLQ (simulated).", err)
		// 在真实生产中，这里应将错误消息推送到“死信队列”(Dead Letter Queue)进行后续分析
		return
	}

	// 2. 重建追踪上下文
	propagator := otel.GetTextMapPropagator()
	header := mq.KafkaHeaderCarrier(msg.Headers)
	parentCtx := propagator.Extract(context.Background(), &header)
	spanOpts := []trace.SpanStartOption{
		trace.WithAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.destination", msg.Topic),
			attribute.String("messaging.kafka.message.key", string(msg.Key)),
			attribute.String("user.id", event.UserID),
			attribute.String("order.trace_id", event.TraceID),
		),
		trace.WithSpanKind(trace.SpanKindConsumer),
	}
	ctx, span := tracer.Start(parentCtx, "order-service.ProcessOrderMessage", spanOpts...)
	defer span.End()

	// 为每个订单的处理流程设置一个独立的超时时间，防止单个订单处理卡死
	processingCtx, cancel := context.WithTimeout(ctx, orderProcessingTimeout)
	defer cancel()

	// 3. 构造本次处理的订单上下文
	orderContext := &orderSvc.OrderContext{
		HTTPClient:          httpClient,
		KafkaNotifyWriter:   kafkaNotifyWriter,
		KafkaDelayWriters:   kafkaDelayWriters,
		KafkaDelayRealTopic: orderTimeoutCheckTopic,
		Ctx:                 processingCtx,
		OrderId:             event.EventId,
		Event:               &event,
	}

	log.Printf("INFO: [Order: %s] Starting verification and reservation process for user %s.", orderContext.OrderId, orderContext.Event.UserID)

	// 4. 执行责任链，进行资源预占和验证
	if err := chain.Handle(orderContext); err != nil {
		log.Printf("ERROR: [Order: %s] Pre-creation process failed: %v. SAGA compensation automatically triggered.", orderContext.OrderId, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "Order creation failed during pre-check")
		// (可选) 发送订单创建失败的通知
		return
	}

	log.Printf("SUCCESS: [Order: %s] All resources reserved. Creating pending payment order in database (simulated).", orderContext.OrderId)
	span.AddEvent("Pending payment order created in DB.")
	// 在此将订单写入数据库，状态为 PENDING_PAYMENT
}
