// cmd/order-service/main.go
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"jaeger-demo/internal/mq"
	"jaeger-demo/internal/tracing"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const (
	serviceName              = "order-service"
	jaegerEndpoint           = "http://localhost:14268/api/traces"
	fraudDetectionServiceURL = "http://localhost:8085/check"
	// <<<<<<< 改造点: 更新服务URL >>>>>>>>>
	inventoryReserveURL = "http://localhost:8082/reserve_stock"
	inventoryReleaseURL = "http://localhost:8082/release_stock"
	pricingServiceURL   = "http://localhost:8084/calculate_price"
	promotionServiceURL = "http://localhost:8087/get_promo_price" // 新增
	shippingServiceURL  = "http://localhost:8086/get_quote"
	// <<<<<<< 改造点结束 >>>>>>>>>
)

var (
	tracer      = otel.Tracer(serviceName)
	kafkaWriter *kafka.Writer
)

const (
	kafkaBrokers      = "localhost:9092"
	notificationTopic = "notifications"
)

type NotificationEvent struct {
	UserID      string `json:"userID"`
	Message     string `json:"message"`
	PromotionID string `json:"promotion_id,omitempty"` // 为个性化通知增加字段
}

func main() {
	tp, err := tracing.InitTracerProvider(serviceName, jaegerEndpoint)
	if err != nil {
		log.Fatalf("failed to initialize tracer provider: %v", err)
	}
	defer tp.Shutdown(context.Background())

	kafkaWriter = mq.NewKafkaWriter([]string{kafkaBrokers}, notificationTopic)
	defer kafkaWriter.Close()

	http.HandleFunc("/create_complex_order", handleCreateComplexOrder)
	log.Println("Order Service listening on :8081")
	log.Fatal(http.ListenAndServe(":8081", nil))
}

func handleCreateComplexOrder(w http.ResponseWriter, r *http.Request) {
	propagator := otel.GetTextMapPropagator()
	ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

	// 从Baggage中读取业务上下文
	b := baggage.FromContext(ctx)
	promoID := b.Member("promotion_id").Value()

	ctx, span := tracer.Start(ctx, "order-service.CreateComplexOrder")
	defer span.End()

	// 生成唯一的订单ID，用于SAGA事务
	orderID := uuid.New().String()

	userID := r.URL.Query().Get("userID")
	isVIP := r.URL.Query().Get("is_vip")
	items := strings.Split(r.URL.Query().Get("items"), ",")
	quantityStr := r.URL.Query().Get("quantity")
	if quantityStr == "" {
		quantityStr = "1"
	}

	span.SetAttributes(
		attribute.String("user.id", userID),
		attribute.Bool("user.is_vip", isVIP == "true"),
		attribute.StringSlice("items", items),
		attribute.String("order.id", orderID),
		attribute.String("promotion.id", promoID), // 记录检测到的活动ID
	)

	// 1. 前置检查 (串行)
	if err := callService(ctx, fraudDetectionServiceURL, r.URL.Query()); err != nil {
		span.RecordError(err)
		http.Error(w, "Fraud check failed", http.StatusBadRequest)
		return
	}

	// <<<<<<< 改造点: SAGA - 预占库存 >>>>>>>>>
	// 2. 库存预占 (循环)
	var reservedItems []string
	for _, item := range items {
		q := url.Values{}
		q.Set("itemID", item)
		q.Set("quantity", quantityStr)
		q.Set("userID", userID)
		q.Set("orderID", orderID) // 传递订单ID
		if err := callService(ctx, inventoryReserveURL, q); err != nil {
			span.RecordError(err)
			// 如果预占失败，启动补偿流程：释放已预占的库存
			compensateStockRelease(ctx, orderID, reservedItems)
			http.Error(w, fmt.Sprintf("Inventory reservation failed for %s", item), http.StatusInternalServerError)
			return
		}
		reservedItems = append(reservedItems, item)
	}
	span.AddEvent("All items reserved successfully", trace.WithAttributes(attribute.StringSlice("reserved_items", reservedItems)))
	// <<<<<<< 改造点结束 >>>>>>>>>

	// 3. 数据聚合 (并行)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)

	// <<<<<<< 改造点: 动态定价 >>>>>>>>>
	go func() {
		defer wg.Done()
		q := url.Values{}
		q.Set("is_vip", isVIP)
		q.Set("user_id", userID)
		// 根据Baggage决定调用哪个服务
		if promoID != "" {
			span.AddEvent("VIP promotion detected, calling promotion-service")
			if err := callService(ctx, promotionServiceURL, q); err != nil {
				errs <- fmt.Errorf("promotion service error: %w", err)
			}
		} else {
			span.AddEvent("No promotion, calling standard pricing-service")
			if err := callService(ctx, pricingServiceURL, q); err != nil {
				errs <- fmt.Errorf("pricing service error: %w", err)
			}
		}
	}()
	// <<<<<<< 改造点结束 >>>>>>>>>

	go func() {
		defer wg.Done()
		if err := callService(ctx, shippingServiceURL, url.Values{}); err != nil {
			errs <- fmt.Errorf("shipping service error: %w", err)
		}
	}()

	wg.Wait()
	close(errs)

	var combinedErr error
	for err := range errs {
		combinedErr = errors.Join(combinedErr, err)
	}

	if combinedErr != nil {
		span.RecordError(combinedErr)
		span.SetStatus(codes.Error, "Downstream service call failed")
		// 聚合服务调用失败，启动补偿流程：释放所有已预占的库存
		compensateStockRelease(ctx, orderID, reservedItems)
		http.Error(w, combinedErr.Error(), http.StatusInternalServerError)
		return
	}

	// 4. 终态处理 (发送个性化通知)
	message := "Your complex order has been successfully created!"
	if promoID != "" {
		message = fmt.Sprintf("Your VIP promotion order (%s) has been successfully created!", promoID)
	}
	err := publishNotificationEvent(ctx, userID, message, promoID)
	if err != nil {
		log.Printf("WARN: Failed to publish notification event: %v", err)
		span.RecordError(err)
	}

	span.AddEvent("Complex order created successfully!")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Complex order created successfully!"))
}

// compensateStockRelease SAGA补偿函数: 释放库存
func compensateStockRelease(ctx context.Context, orderID string, items []string) {
	ctx, span := tracer.Start(ctx, "order-service.CompensateStockRelease")
	defer span.End()

	span.SetAttributes(
		attribute.String("order.id", orderID),
		attribute.StringSlice("items.to.release", items),
		attribute.Bool("compensation.logic", true),
	)

	log.Printf("Starting compensation logic: releasing stock for order %s", orderID)

	var wg sync.WaitGroup
	for _, item := range items {
		wg.Add(1)
		go func(it string) {
			defer wg.Done()
			q := url.Values{}
			q.Set("itemID", it)
			q.Set("orderID", orderID)
			// 注意: 补偿逻辑的失败处理在真实世界中会更复杂（例如持久化后重试）
			if err := callService(ctx, inventoryReleaseURL, q); err != nil {
				log.Printf("ERROR: Stock release compensation failed for item %s: %v", it, err)
				span.RecordError(err)
			}
		}(item)
	}
	wg.Wait()
	span.AddEvent("Compensation logic finished.")
}

func publishNotificationEvent(ctx context.Context, userID, message, promoID string) error {
	ctx, span := tracer.Start(ctx, "order-service.PublishNotification")
	defer span.End()

	span.SetAttributes(
		attribute.String("messaging.system", "kafka"),
		attribute.String("messaging.destination", notificationTopic),
		attribute.String("user.id", userID),
		attribute.String("promotion.id", promoID),
	)

	event := NotificationEvent{
		UserID:      userID,
		Message:     message,
		PromotionID: promoID,
	}
	eventBytes, err := json.Marshal(event)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to marshal notification event: %w", err)
	}

	err = mq.ProduceMessage(ctx, kafkaWriter, []byte(userID), eventBytes)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to produce kafka message: %w", err)
	}

	span.AddEvent("Notification event successfully published to Kafka.")
	return nil
}

// callService 函数基本保持不变
func callService(ctx context.Context, serviceURL string, params url.Values) error {
	parsedURL, err := url.Parse(serviceURL)
	if err != nil {
		return err
	}
	spanName := fmt.Sprintf("call-%s", strings.Split(parsedURL.Host, ":")[0])
	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

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

	span.SetAttributes(
		attribute.String("http.url", downstreamURL.String()),
		attribute.String("http.method", "POST"),
	)
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("service %s returned status %s", serviceURL, resp.Status)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}
