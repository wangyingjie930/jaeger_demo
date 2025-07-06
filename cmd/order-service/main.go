// cmd/order-service/main.go
package main

import (
	"context"
	"jaeger-demo/internal/pkg/httpclient"
	"jaeger-demo/internal/pkg/mq"
	"jaeger-demo/internal/pkg/redis"
	"jaeger-demo/internal/pkg/tracing"
	handler "jaeger-demo/internal/service/order"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
)

const (
	serviceName       = "order-service"
	notificationTopic = "notifications"
	requestTimeout    = 10 * time.Second
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

	redisClient, err := redis.NewClient(redisAddrs)
	if err != nil {
		log.Fatalf("failed to initialize redis client: %v", err)
	}

	// ✨ [核心改造] 初始化业务 Service
	seckillService := handler.NewSeckillService(redisClient)

	// (可选, 用于测试) 准备一个秒杀商品
	err = seckillService.PrepareSeckillProduct(context.Background(), "product_123", 100)
	if err != nil {
		log.Printf("WARN: could not prepare seckill product for testing: %v", err)
	}

	// 2. 构建责任链 (将依赖项预先绑定)
	// 这是一个最佳实践，将依赖注入和业务链的构建分离
	orderChain := buildOrderProcessingChain(seckillService)

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
func buildOrderProcessingChain(seckillSvc *handler.SeckillService) handler.Handler {
	orderHandler := new(handler.TransactionHandler)
	orderHandler.SetNext(handler.NewSeckillHandler(seckillSvc)). // 注入 SeckillService
									SetNext(new(handler.FraudCheckHandler)).
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
	// ✨ [核心改造点]
	// 1. 从原始请求中提取 Trace 和 Baggage 上下文
	propagator := otel.GetTextMapPropagator()
	// 使用原始请求的上下文作为父上下文
	parentCtx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

	// 2. 创建一个带有超时的子上下文
	// 整个订单处理流程（从进入责任链到结束）必须在 requestTimeout 内完成
	ctx, cancel := context.WithTimeout(parentCtx, requestTimeout)
	defer cancel() // 确保在函数退出时释放资源，无论成功还是失败

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
		// 对于其他错误，记录日志即可，因为具体的HTTP响应已在链中处理
		log.Printf("ERROR: Order processing chain failed for order %s: %v", orderContext.OrderId, err)

		// ✨ [错误处理]
		// 检查错误是否是由于上下文超时引起的
		if ctx.Err() == context.DeadlineExceeded {
			// 如果是超时，记录特定的日志并返回一个更友好的错误给客户端
			span.SetStatus(codes.Error, "Request timed out")
			span.RecordError(err)
			http.Error(w, "Request timed out", http.StatusGatewayTimeout) // 504 Gateway Timeout 是一个合适的HTTP状态码
		} else {
			span.RecordError(err)
		}
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
