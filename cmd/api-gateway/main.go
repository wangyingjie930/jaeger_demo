package main

import (
	"context"
	"io"
	"jaeger-demo/internal/tracing"
	"log"
	"net/http"
	"net/url"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
)

const (
	serviceName         = "api-gateway"
	jaegerEndpoint      = "http://localhost:14268/api/traces"
	orderServiceBaseURL = "http://localhost:8081"
)

func main() {
	tp, err := tracing.InitTracerProvider(serviceName, jaegerEndpoint)
	if err != nil {
		log.Fatalf("failed to initialize tracer provider: %v", err)
	}
	defer tp.Shutdown(context.Background())

	// 新增的复杂订单路由
	http.HandleFunc("/create_complex_order", complexOrderHandler)
	log.Println("API Gateway listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func complexOrderHandler(w http.ResponseWriter, r *http.Request) {
	tracer := otel.Tracer(serviceName)
	ctx, span := tracer.Start(r.Context(), "api-gateway.ComplexOrderHandler")
	defer span.End()

	// 构建向下游服务传递的 URL，携带所有查询参数
	downstreamURL, _ := url.Parse(orderServiceBaseURL + "/create_complex_order")
	downstreamURL.RawQuery = r.URL.RawQuery // 直接复制所有查询参数

	span.SetAttributes(
		attribute.String("http.method", "GET"),
		attribute.String("downstream.url", downstreamURL.String()),
	)

	req, err := http.NewRequestWithContext(ctx, "POST", downstreamURL.String(), nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 注入追踪上下文
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}
