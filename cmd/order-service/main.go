package main

import (
	"context"
	"errors"
	"fmt"
	"jaeger-demo/internal/tracing"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
)

const (
	serviceName              = "order-service"
	jaegerEndpoint           = "http://localhost:14268/api/traces"
	fraudDetectionServiceURL = "http://localhost:8085/check"
	inventoryServiceURL      = "http://localhost:8082/check_stock"
	pricingServiceURL        = "http://localhost:8084/calculate_price"
	shippingServiceURL       = "http://localhost:8086/get_quote"
	notificationServiceURL   = "http://localhost:8083/send_notification"
)

var tracer = otel.Tracer(serviceName)

func main() {
	tp, err := tracing.InitTracerProvider(serviceName, jaegerEndpoint)
	if err != nil {
		log.Fatalf("failed to initialize tracer provider: %v", err)
	}
	defer tp.Shutdown(context.Background())

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

	// 3. 数据聚合 (并行)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)

	go func() {
		defer wg.Done()
		q := url.Values{}
		q.Set("is_vip", isVIP)
		if err := callService(ctx, pricingServiceURL, q); err != nil {
			errs <- fmt.Errorf("pricing service error: %w", err)
		}
	}()

	go func() {
		defer wg.Done()
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

	// 4. 终态处理
	if err := callService(ctx, notificationServiceURL, r.URL.Query()); err != nil {
		span.RecordError(err)
		http.Error(w, "Failed to send notification", http.StatusInternalServerError)
		return
	}

	span.AddEvent("Complex order created successfully!")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Complex order created successfully!"))
}

// callService 完整实现
func callService(ctx context.Context, serviceURL string, params url.Values) error {
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
