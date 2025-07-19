// cmd/pricing-service/main.go
package main

import (
	"fmt"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"nexus/internal/pkg/bootstrap"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
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

const serviceName = "pricing-service"

var (
	tracer trace.Tracer
)

func main() {
	bootstrap.Init()

	bootstrap.StartService(bootstrap.AppInfo{
		ServiceName: serviceName,
		Port:        8084,
		RegisterHandlers: func(ctx bootstrap.AppCtx) {
			tracer = otel.Tracer(serviceName)
			ctx.Mux.HandleFunc("/calculate_price", handleCalculatePrice)
		},
	})
}

func handleCalculatePrice(w http.ResponseWriter, r *http.Request) {
	ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
	ctx, span := tracer.Start(ctx, "pricing-service.CalculatePrice")
	defer span.End()

	isVIP := r.URL.Query().Get("is_vip")
	userID := r.URL.Query().Get("user_id")
	span.SetAttributes(attribute.Bool("user.is_vip", isVIP == "true"))

	// <<<<<<< 复杂故障注入点 >>>>>>>>>
	if userID == "user-normal-456" {
		// 模拟复杂计算导致的超时
		time.Sleep(600 * time.Millisecond)
		err := fmt.Errorf("random error")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// <<<<<<< 故障注入结束 >>>>>>>>>

	// 正常逻辑
	time.Sleep(150 * time.Millisecond) // 模拟正常计算耗时
	span.AddEvent("Standard price calculated")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"price": 99.99}`))
}
