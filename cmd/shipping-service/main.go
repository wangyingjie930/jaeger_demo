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
	serviceName    = "shipping-service"
	jaegerEndpoint = "http://localhost:14268/api/traces"
)

var tracer = otel.Tracer(serviceName)

func main() {
	tp, _ := tracing.InitTracerProvider(serviceName, jaegerEndpoint)
	defer tp.Shutdown(context.Background())
	http.HandleFunc("/get_quote", handleGetQuote)
	log.Println("Shipping Service listening on :8086")
	log.Fatal(http.ListenAndServe(":8086", nil))
}
func handleGetQuote(w http.ResponseWriter, r *http.Request) {
	ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
	_, span := tracer.Start(ctx, "shipping-service.GetQuote")
	defer span.End()
	log.Println("Calculating shipping quote...")
	time.Sleep(200 * time.Millisecond)
	span.AddEvent("Shipping quote calculated")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"cost": 10.0}`))
}
