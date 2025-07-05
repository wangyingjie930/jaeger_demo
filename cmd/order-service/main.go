// cmd/order-service/main.go
package main

import (
	"context"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/propagation"
	"jaeger-demo/internal/pkg/httpclient"
	"jaeger-demo/internal/pkg/mq"
	"jaeger-demo/internal/pkg/tracing"
	handler "jaeger-demo/internal/service/order"
	"log"
	"net/http"
	"os"
	"strings"
)

const (
	serviceName       = "order-service"
	notificationTopic = "notifications"
)

// 从环境变量或配置中读取所有下游服务的URL
var (
	jaegerEndpoint = getEnv("JAEGER_ENDPOINT", "http://localhost:14268/api/traces")
	kafkaBrokers   = getEnv("KAFKA_BROKERS", "localhost:9092")
)

// main 函数是应用的“组装根” (Composition Root)
// 它的核心职责是：创建并组装所有依赖项，然后启动应用。
func main() {
	// 1. 初始化核心技术组件
	tp, err := tracing.InitTracerProvider(serviceName, jaegerEndpoint)
	if err != nil {
		log.Fatalf("failed to initialize tracer provider: %v", err)
	}
	defer tp.Shutdown(context.Background())

	tracer := otel.Tracer(serviceName)

	httpClient := httpclient.NewClient(tracer)

	kafkaWriter := mq.NewKafkaWriter(strings.Split(kafkaBrokers, ","), notificationTopic)
	defer kafkaWriter.Close()

	// 2. 构建责任链 (将依赖项预先绑定)
	// 这是一个最佳实践，将依赖注入和业务链的构建分离
	orderChain := buildOrderProcessingChain()

	// 3. 设置HTTP路由
	// 使用闭包将已创建的依赖项 (httpClient, kafkaWriter, orderChain) 传递给 Handler
	http.HandleFunc("/create_complex_order", func(w http.ResponseWriter, r *http.Request) {
		handleCreateOrder(w, r, httpClient, kafkaWriter, orderChain)
	})

	http.HandleFunc("/healthz", healthzHandler)
	http.Handle("/metrics", promhttp.Handler())

	// 4. 启动服务
	log.Printf("✅ Order Service is running on :8081, using Chain of Responsibility pattern.")
	log.Fatal(http.ListenAndServe(":8081", nil))
}

// buildOrderProcessingChain 负责构建和连接责任链中的所有处理器
func buildOrderProcessingChain() handler.Handler {
	orderHandler := new(handler.TransactionHandler)
	orderHandler.SetNext(new(handler.FraudCheckHandler)).
		SetNext(new(handler.InventoryReserveHandler)).
		SetNext(new(handler.PriceHandler)).
		SetNext(new(handler.NotificationHandler))

	return orderHandler
}

// handleCreateOrder 是每个HTTP请求的入口
func handleCreateOrder(
	w http.ResponseWriter,
	r *http.Request,
	httpClient *httpclient.Client,
	kafkaWriter *kafka.Writer,
	chain handler.Handler,
) {
	// 1. 提取 Trace 和 Baggage 上下文
	propagator := otel.GetTextMapPropagator()
	ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
	ctx, span := httpClient.Tracer.Start(ctx, "order-service.http_request")
	defer span.End()

	b := baggage.FromContext(ctx)
	promoId := b.Member("promotion_id").Value()
	userID := r.URL.Query().Get("userId")

	span.SetAttributes(
		attribute.String("user.id", userID),
		attribute.String("pattern", "Chain of Responsibility + SAGA"),
	)

	// 2. 为当前请求创建唯一的订单上下文 (OrderContext)
	// 这是责任链中传递所有状态和依赖的核心对象
	orderContext := &handler.OrderContext{
		HTTPClient:  httpClient,
		KafkaWriter: kafkaWriter,
		Ctx:         ctx,
		Writer:      w,
		Request:     r,
		Params:      r.URL.Query(),
		OrderId:     uuid.New().String(),
		UserId:      userID,
		IsVIP:       r.URL.Query().Get("is_vip") == "true",
		Items:       strings.Split(r.URL.Query().Get("items"), ","),
		PromoId:     promoId,
	}

	// 3. 启动责任链
	if err := chain.Handle(orderContext); err != nil {
		// 错误已经在链中被处理（例如返回HTTP Error），这里只需记录日志即可
		log.Printf("ERROR: Order processing chain failed for order %s: %v", orderContext.OrderId, err)
		span.RecordError(err)
	} else {
		log.Printf("INFO: Order processing chain completed successfully for order %s", orderContext.OrderId)
	}
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
