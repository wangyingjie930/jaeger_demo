// cmd/order-service/main.go
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"jaeger-demo/internal/mq" // 新增
	"jaeger-demo/internal/tracing"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/segmentio/kafka-go" // 新增
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const (
	serviceName              = "order-service"
	jaegerEndpoint           = "http://localhost:14268/api/traces"
	fraudDetectionServiceURL = "http://localhost:8085/check"
	inventoryServiceURL      = "http://localhost:8082/check_stock"
	pricingServiceURL        = "http://localhost:8084/calculate_price"
	shippingServiceURL       = "http://localhost:8086/get_quote"
	// notificationServiceURL 不再需要
	// notificationServiceURL   = "http://localhost:8083/send_notification"
)

var (
	tracer            = otel.Tracer(serviceName)
	kafkaBrokers      = []string{"localhost:9092"} // Kafka Broker 地址
	notificationTopic = "notifications"            // Kafka Topic
	kafkaWriter       *kafka.Writer                // Kafka 生产者
)

// NotificationEvent 定义了要发送到 Kafka 的消息结构
type NotificationEvent struct {
	UserID  string `json:"userID"`
	Message string `json:"message"`
}

func main() {
	tp, err := tracing.InitTracerProvider(serviceName, jaegerEndpoint)
	if err != nil {
		log.Fatalf("failed to initialize tracer provider: %v", err)
	}
	defer tp.Shutdown(context.Background())

	// 初始化 Kafka 生产者
	kafkaWriter = mq.NewKafkaWriter(kafkaBrokers, notificationTopic)
	defer kafkaWriter.Close()
	log.Println("Kafka writer initialized for topic:", notificationTopic)

	http.HandleFunc("/create_complex_order", handleCreateComplexOrder)
	log.Println("Order Service listening on :8081")
	log.Fatal(http.ListenAndServe(":8081", nil))
}

func handleCreateComplexOrder(w http.ResponseWriter, r *http.Request) {
	ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
	ctx, span := tracer.Start(ctx, "order-service.CreateComplexOrder")
	defer span.End()

	userID := r.URL.Query().Get("userID")
	isVIP := r.URL.Query().Get("is_vip")
	items := strings.Split(r.URL.Query().Get("items"), ",")
	span.SetAttributes(
		attribute.String("user.id", userID),
		attribute.Bool("user.is_vip", isVIP == "true"),
		attribute.StringSlice("items", items),
	)

	// 1. 前置检查 (串行)
	if err := callService(ctx, fraudDetectionServiceURL, r.URL.Query()); err != nil {
		span.RecordError(err)
		http.Error(w, "Fraud check failed", http.StatusBadRequest)
		return
	}

	span.AddEvent("Starting inventory check process")

	// 2. 库存检查 (循环)
	for _, item := range items {
		q := url.Values{}
		q.Set("itemID", item)
		q.Set("quantity", "1")
		if err := callService(ctx, inventoryServiceURL, q); err != nil {
			span.RecordError(err)
			http.Error(w, fmt.Sprintf("Inventory check failed for %s", item), http.StatusInternalServerError)
			return
		}
	}

	span.AddEvent("Inventory check completed successfully")

	// 3. 数据聚合 (并行)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)

	go func() {
		defer wg.Done()
		span.AddEvent("Starting pricing and shipping calculation")
		q := url.Values{}
		q.Set("is_vip", isVIP)
		if err := callService(ctx, pricingServiceURL, q); err != nil {
			errs <- fmt.Errorf("pricing service error: %w", err)
		}
	}()

	go func() {
		defer wg.Done()
		span.AddEvent("Pricing and shipping calculation finished")
		q := url.Values{}
		if err := callService(ctx, shippingServiceURL, q); err != nil {
			errs <- fmt.Errorf("shipping service error: %w", err)
		}
	}()

	wg.Wait()
	close(errs)

	var combinedErr error
	for err := range errs {
		combinedErr = errors.Join(combinedErr, err)
		span.RecordError(err)
	}
	if combinedErr != nil {
		http.Error(w, combinedErr.Error(), http.StatusInternalServerError)
		return
	}

	// 4. 终态处理 (改造点)
	// 不再直接调用 notification-service，而是发送 Kafka 消息
	err := publishNotificationEvent(ctx, userID, "Your complex order has been successfully created!")
	if err != nil {
		// 在真实世界中，这里应该有重试或降级逻辑
		log.Printf("WARN: Failed to publish notification event: %v", err)
		span.RecordError(err)
		// 注意：即使通知失败，我们也不再阻塞主流程或返回错误给用户
	}

	span.AddEvent("Complex order created successfully! Notification event published.")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Complex order created successfully!"))
}

// publishNotificationEvent 创建并发送通知事件到 Kafka
func publishNotificationEvent(ctx context.Context, userID, message string) error {
	// 创建一个新的 Span 来描述这个异步发布操作
	ctx, span := tracer.Start(ctx, "order-service.PublishNotification")
	defer span.End()

	span.SetAttributes(
		attribute.String("messaging.system", "kafka"),
		attribute.String("messaging.destination", notificationTopic),
		attribute.String("user.id", userID),
	)

	// 验证追踪上下文是否有效
	if spanCtx := trace.SpanContextFromContext(ctx); spanCtx.IsValid() {
		log.Printf("Publishing notification with valid trace context: traceID=%s, spanID=%s",
			spanCtx.TraceID().String(), spanCtx.SpanID().String())
	} else {
		log.Printf("Warning: Publishing notification without valid trace context")
	}

	event := NotificationEvent{
		UserID:  userID,
		Message: message,
	}
	eventBytes, err := json.Marshal(event)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to marshal notification event: %w", err)
	}

	// 使用封装好的函数来发送消息，它会自动注入追踪上下文
	err = mq.ProduceMessage(ctx, kafkaWriter, []byte(userID), eventBytes)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to produce kafka message: %w", err)
	}

	span.AddEvent("Notification event successfully published to Kafka.")
	return nil
}

// callService 函数保持不变
func callService(ctx context.Context, serviceURL string, params url.Values) error {
	// ... (此函数代码不变)
	parsedURL, err := url.Parse(serviceURL)
	if err != nil {
		return err
	}
	// 创建一个描述性的 Span 名称
	spanName := fmt.Sprintf("call-%s", parsedURL.Hostname())
	ctx, span := tracer.Start(ctx, spanName)
	defer span.End()

	// 添加目标 URL 属性
	span.SetAttributes(attribute.String("http.url", serviceURL))

	downstreamURL := *parsedURL
	q := downstreamURL.Query()
	for key, values := range params {
		for _, value := range values {
			q.Add(key, value)
		}
	}
	downstreamURL.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "POST", downstreamURL.String(), nil)
	if err != nil {
		span.RecordError(err)
		return err
	}

	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		span.RecordError(err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("service %s returned status %d", serviceURL, resp.StatusCode)
		span.RecordError(err)
		return err
	}
	return nil
}
