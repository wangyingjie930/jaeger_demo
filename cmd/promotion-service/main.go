// cmd/promotion-service/main.go
package main

import (
	"context"
	"fmt"
	"jaeger-demo/internal/tracing"
	"log"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/propagation"
)

const (
	serviceName    = "promotion-service"
	jaegerEndpoint = "http://localhost:14268/api/traces"
)

var tracer = otel.Tracer(serviceName)

func main() {
	tp, _ := tracing.InitTracerProvider(serviceName, jaegerEndpoint)
	defer tp.Shutdown(context.Background())
	http.HandleFunc("/get_promo_price", handleGetPromoPrice)
	log.Println("Promotion Service listening on :8087")
	log.Fatal(http.ListenAndServe(":8087", nil))
}

func handleGetPromoPrice(w http.ResponseWriter, r *http.Request) {
	// 依然先提取通用的追踪上下文
	propagator := otel.GetTextMapPropagator()
	ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

	// 从Baggage中提取业务上下文
	promoID := baggage.FromContext(ctx).Member("promotion_id").Value()

	ctx, span := tracer.Start(ctx, "promotion-service.GetPromoPrice")
	defer span.End()

	// 将业务信息记录到Span的Attribute中，便于排查
	span.SetAttributes(
		attribute.String("user.is_vip", r.URL.Query().Get("is_vip")),
		attribute.String("promotion.id", promoID),
	)

	log.Printf("Calculating promo price for promotion '%s'", promoID)

	// 模拟促销价格计算的耗时
	time.Sleep(120 * time.Millisecond)

	if promoID == "" {
		err := fmt.Errorf("promotion_id is missing from baggage")
		span.RecordError(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	span.AddEvent("Promotion price calculated successfully")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"price": 79.99, "promo": "` + promoID + `"}`))
}
