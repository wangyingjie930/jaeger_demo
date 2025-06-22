// cmd/inventory-service/main.go
package main

import (
	"context"
	"fmt"
	"jaeger-demo/internal/tracing"
	"log"
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
)

const (
	serviceName    = "inventory-service"
	jaegerEndpoint = "http://localhost:14268/api/traces"
)

var tracer = otel.Tracer(serviceName)

func main() {
	tp, err := tracing.InitTracerProvider(serviceName, jaegerEndpoint)
	if err != nil {
		log.Fatalf("failed to initialize tracer provider: %v", err)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}()

	http.HandleFunc("/check_stock", checkStockHandler)
	log.Println("Inventory Service listening on :82")
	log.Fatal(http.ListenAndServe(":8082", nil))
}

func checkStockHandler(w http.ResponseWriter, r *http.Request) {
	propagator := otel.GetTextMapPropagator()
	ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
	ctx, span := tracer.Start(ctx, "inventory-service.CheckStock")
	defer span.End()

	itemID := r.URL.Query().Get("itemID")
	quantityStr := r.URL.Query().Get("quantity")
	quantity, _ := strconv.Atoi(quantityStr)

	span.SetAttributes(
		attribute.String("item.id", itemID),
		attribute.Int("item.quantity", quantity),
	)

	// <<<<<<< 故障注入点 >>>>>>>>>
	if itemID == "item-faulty-123" && quantity > 10 {
		log.Printf("Injecting fault for item %s", itemID)
		time.Sleep(500 * time.Millisecond) // 模拟耗时

		err := fmt.Errorf("inventory check failed for faulty item %s", itemID)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		http.Error(w, "Inventory service unavailable for this item", http.StatusInternalServerError)
		return
	}
	// <<<<<<< 故障注入结束 >>>>>>>>>

	log.Printf("Stock check successful for item %s", itemID)
	span.AddEvent("Stock check successful")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Stock available"))
}
