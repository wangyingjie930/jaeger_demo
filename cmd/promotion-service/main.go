// cmd/promotion-service/main.go
package main

import (
	"fmt"
	"go.opentelemetry.io/otel/trace"
	"log"
	"net/http"
	"nexus/internal/pkg/bootstrap"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
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
	serviceName = "promotion-service"
)

var (
	tracer trace.Tracer
)

func main() {
	bootstrap.StartService(bootstrap.AppInfo{
		ServiceName: serviceName,
		Port:        8087,
		RegisterHandlers: func(ctx bootstrap.AppCtx) {
			tracer = otel.Tracer(serviceName)
			ctx.Mux.HandleFunc("/get_promo_price", handleGetPromoPrice)
		},
	})
}

func handleGetPromoPrice(w http.ResponseWriter, r *http.Request) {
	// 依然先提取通用的追踪上下文
	propagator := otel.GetTextMapPropagator()
	ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

	// 从Baggage中提取业务上下文
	promoId := baggage.FromContext(ctx).Member("promotion_id").Value()

	ctx, span := tracer.Start(ctx, "promotion-service.GetPromoPrice")
	defer span.End()

	// 将业务信息记录到Span的Attribute中，便于排查
	span.SetAttributes(
		attribute.String("user.is_vip", r.URL.Query().Get("is_vip")),
		attribute.String("promotion.id", promoId),
	)

	log.Printf("Calculating promo price for promotion '%s'", promoId)

	// 模拟促销价格计算的耗时
	time.Sleep(120 * time.Millisecond)

	if promoId == "" {
		err := fmt.Errorf("promotion_id is missing from baggage")
		span.RecordError(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	span.AddEvent("Promotion price calculated successfully")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"price": 79.99, "promo": "` + promoId + `"}`))
}
