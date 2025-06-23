package main

import (
	"context"
	"jaeger-demo/internal/tracing"
	"log"
	"net/http"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// getEnv 从环境变量中读取配置。
// 如果环境变量不存在，则返回提供的默认值。
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

const (
	serviceName = "fraud-detection-service"
)

var (
	jaegerEndpoint = getEnv("JAEGER_ENDPOINT", "http://localhost:14268/api/traces")
	tracer         = otel.Tracer(serviceName)
)

func main() {
	tp, _ := tracing.InitTracerProvider(serviceName, jaegerEndpoint)
	defer tp.Shutdown(context.Background())
	http.HandleFunc("/check", handleFraudCheck)
	log.Println("Fraud Detection Service listening on :8085")
	log.Fatal(http.ListenAndServe(":8085", nil))
}
func handleFraudCheck(w http.ResponseWriter, r *http.Request) {
	ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
	_, span := tracer.Start(ctx, "fraud-detection-service.Check")
	defer span.End()
	log.Println("Performing fraud check...")
	time.Sleep(80 * time.Millisecond)
	span.AddEvent("Fraud check passed")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Fraud check passed"))
}
