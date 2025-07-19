// cmd/order-service/main.go
package main

import (
	"context"
	"nexus/internal/pkg/bootstrap"
	"nexus/internal/pkg/httpclient"
	"nexus/internal/pkg/mq"
	"nexus/internal/service/order/application"
	"nexus/internal/service/order/infrastructure"
	"nexus/internal/service/order/infrastructure/adapter"
	"nexus/internal/service/order/interfaces"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
)

const (
	serviceName                  = "order-service"
	notificationTopic            = "notifications"
	orderCreationTopic           = "order-creation-topic"
	orderCreationConsumerGroupID = "order-creation-consumer-group"
	orderTimeoutCheckTopic       = "order-timeout-check-topic"
	delayTopics                  = "delay_topic_5s" // 示例延迟主题，应从配置读取
	timeoutCheckConsumerGroupID  = "timeout-check-consumer-group"
)

// main 函数是应用的"组装根" (Composition Root)
// 它的核心职责是: 创建并组装所有依赖项，然后启动应用。
func main() {
	bootstrap.Init()
	tracer := otel.Tracer(serviceName)
	brokers := strings.Split(bootstrap.GetCurrentConfig().Infra.Kafka.Brokers, ",")

	notificationKafkaWriter := mq.NewKafkaWriter(brokers, notificationTopic)
	defer notificationKafkaWriter.Close()

	delayKafkaWriter := mq.NewKafkaWriter(brokers, delayTopics)
	defer delayKafkaWriter.Close()

	orderCreationWriter := mq.NewKafkaWriter(brokers, orderCreationTopic)
	defer orderCreationWriter.Close()

	orderCreationReader := mq.NewKafkaReader(brokers, orderCreationTopic, orderCreationConsumerGroupID)
	defer orderCreationReader.Close()

	timeoutCheckReader := mq.NewKafkaReader(brokers, orderTimeoutCheckTopic, timeoutCheckConsumerGroupID)
	defer timeoutCheckReader.Close()

	orderRepo := infrastructure.NewMysqlRepository()

	bootstrap.StartService(bootstrap.AppInfo{
		ServiceName: serviceName,
		Port:        8081,
		RegisterHandlers: func(appCtx bootstrap.AppCtx) {

			httpClient := httpclient.NewClient(tracer, appCtx.Nacos) // 此处 Nacos 客户端为 nil，因为原始代码中它未被用于服务发现
			fraudAdapter := adapter.NewFraudHTTPAdapter(httpClient)
			inventoryAdapter := adapter.NewInventoryHTTPAdapter(httpClient)
			pricingAdapter := adapter.NewPricingHTTPAdapter(httpClient) // 同时实现了 PricingService 和 ShippingService
			schedulerAdapter := adapter.NewSchedulerKafkaAdapter(delayKafkaWriter)
			notificationAdapter := adapter.NewNotificationKafkaAdapter(notificationKafkaWriter)

			appService := application.NewOrderApplicationService(
				orderRepo,
				5*time.Second,
				otel.Tracer(serviceName),
				infrastructure.NewOrderProducerAdapter(orderCreationWriter),
				fraudAdapter,
				inventoryAdapter,
				pricingAdapter,
				pricingAdapter,
				schedulerAdapter,
				notificationAdapter,
			)
			httpHandler := interfaces.NewOrderHandler(appService)

			httpHandler.RegisterRoutes(appCtx.Mux)

			infrastructure.NewOrderTimeOutConsumerAdapter(timeoutCheckReader, appService).Start(context.Background())
			infrastructure.NewOrderConsumerAdapter(orderCreationReader, appService).Start(context.Background())
		},
	})
}
