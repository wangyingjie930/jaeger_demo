package main

import (
	"context"
	"jaeger-demo/internal/tracing"
	"log"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

const (
	serviceName    = "fraud-detection-service"
	jaegerEndpoint = "http://localhost:14268/api/traces"
)

var tracer = otel.Tracer(serviceName)

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
