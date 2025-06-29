package main

import (
	"context"
	"io"
	"jaeger-demo/internal/tracing"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/propagation"
)

const (
	serviceName = "api-gateway"
)

var (
	jaegerEndpoint      = getEnv("JAEGER_ENDPOINT", "http://localhost:14268/api/traces")
	orderServiceBaseURL = getEnv("ORDER_SERVICE_BASE_URL", "http://localhost:8081")
)

// <<<<<<< 新增特性开关 >>>>>>>>>
var featureFlags = map[string]bool{
	"PROMO_VIP_SUMMER_2025": true, // 硬编码一个特性开关
}

// <<<<<<< 新增特性开关 >>>>>>>>>

func main() {
	tp, err := tracing.InitTracerProvider(serviceName, jaegerEndpoint)
	if err != nil {
		log.Fatalf("failed to initialize tracer provider: %v", err)
	}
	defer tp.Shutdown(context.Background())

	http.Handle("/metrics", promhttp.Handler())

	// <<<<<<< 新增: 注册健康检查路由 >>>>>>>>>
	http.HandleFunc("/healthz", healthzHandler)
	http.HandleFunc("/readyz", readyzHandler)
	// <<<<<<< 新增结束 >>>>>>>>>

	http.HandleFunc("/create_complex_order", complexOrderHandler)
	log.Println("API Gateway listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func complexOrderHandler(w http.ResponseWriter, r *http.Request) {
	tracer := otel.Tracer(serviceName)
	// 初始上下文
	ctx := r.Context()

	// <<<<<<< 改造点: Feature Flag 和 Baggage 注入 >>>>>>>>>
	isVIP := r.URL.Query().Get("is_vip") == "true"
	isPromotionActive := featureFlags["PROMO_VIP_SUMMER_2025"]

	ctx, span := tracer.Start(ctx, "api-gateway.ComplexOrderHandler")

	span.SetAttributes(
		attribute.Bool("user.is_vip", isVIP),
		attribute.Bool("feature_flag.PROMO_VIP_SUMMER_2025", isPromotionActive),
	)

	// 如果是VIP用户且活动开启，则通过Baggage向下游传递业务信息
	if isVIP && isPromotionActive {
		log.Println("VIP user detected, activating promotion baggage.")
		promoBaggage, _ := baggage.NewMember("promotion_id", "VIP_SUMMER_SALE")
		b, _ := baggage.FromContext(ctx).SetMember(promoBaggage)
		ctx = baggage.ContextWithBaggage(ctx, b)
		span.AddEvent("Baggage with promotion_id injected.")
	}
	// <<<<<<< 改造点结束 >>>>>>>>>
	defer span.End()

	downstreamURL, _ := url.Parse(orderServiceBaseURL + "/create_complex_order")
	downstreamURL.RawQuery = r.URL.RawQuery

	span.SetAttributes(
		attribute.String("http.method", "GET"),
		attribute.String("downstream.url", downstreamURL.String()),
	)

	req, err := http.NewRequestWithContext(ctx, "POST", downstreamURL.String(), nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 注入追踪上下文(包括Baggage)
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

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// <<<<<<< 新增: 健康检查 Handler >>>>>>>>>
func healthzHandler(w http.ResponseWriter, r *http.Request) {
	// 对于 livenessProbe，只要能响应，就说明进程存活
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func readyzHandler(w http.ResponseWriter, r *http.Request) {
	// 对于 readinessProbe，可以加入对下游服务的检查
	// 这里我们用一个简化的例子：检查 order-service 是否可达
	// 在真实应用中，你可能需要检查所有关键依赖
	resp, err := http.Get(orderServiceBaseURL + "/healthz") // 假设 order-service 也实现了 /healthz
	if err != nil || resp.StatusCode != http.StatusOK {
		http.Error(w, "Downstream order-service not ready", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// <<<<<<< 新增结束 >>>>>>>>>
