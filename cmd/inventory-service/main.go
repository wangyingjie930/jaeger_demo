// cmd/inventory-service/main.go
package main

import (
	"context"
	"fmt"
	"jaeger-demo/internal/tracing"
	"log"
	"net/http"
	"os"
	"strconv"
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

const (
	serviceName = "inventory-service"
)

var (
	jaegerEndpoint = getEnv("JAEGER_ENDPOINT", "http://localhost:14268/api/traces")
	tracer         = otel.Tracer(serviceName)
)

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

	// <<<<<<< 改造点: 增加SAGA相关接口 >>>>>>>>>
	http.HandleFunc("/check_stock", checkStockHandler)
	http.HandleFunc("/reserve_stock", reserveStockHandler) // 新增：预占库存
	http.HandleFunc("/release_stock", releaseStockHandler) // 新增：释放库存
	// <<<<<<< 改造点结束 >>>>>>>>>

	log.Println("Inventory Service listening on :8082")
	log.Fatal(http.ListenAndServe(":8082", nil))
}

// reserveStockHandler 模拟预占库存
func reserveStockHandler(w http.ResponseWriter, r *http.Request) {
	propagator := otel.GetTextMapPropagator()
	ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
	ctx, span := tracer.Start(ctx, "inventory-service.ReserveStock")
	defer span.End()

	itemID := r.URL.Query().Get("itemID")
	quantityStr := r.URL.Query().Get("quantity")
	quantity, _ := strconv.Atoi(quantityStr)
	orderID := r.URL.Query().Get("orderID") // 预占和释放需要关联一个唯一标识

	span.SetAttributes(
		attribute.String("item.id", itemID),
		attribute.Int("item.quantity", quantity),
		attribute.String("order.id", orderID),
	)

	// 故障注入点
	if itemID == "item-faulty-123" && quantity > 10 {
		log.Printf("Injecting fault for item %s", itemID)
		time.Sleep(500 * time.Millisecond)

		err := fmt.Errorf("inventory reserve failed for faulty item %s", itemID)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		http.Error(w, "Inventory service unavailable for this item", http.StatusInternalServerError)
		return
	}

	log.Printf("Stock reservation successful for item %s, order %s", itemID, orderID)
	span.AddEvent("Stock reserved")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Stock reserved"))
}

// releaseStockHandler 模拟释放库存
func releaseStockHandler(w http.ResponseWriter, r *http.Request) {
	propagator := otel.GetTextMapPropagator()
	ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
	ctx, span := tracer.Start(ctx, "inventory-service.ReleaseStock (Compensation)")
	defer span.End()

	itemID := r.URL.Query().Get("itemID")
	orderID := r.URL.Query().Get("orderID")

	span.SetAttributes(
		attribute.String("item.id", itemID),
		attribute.String("order.id", orderID),
		attribute.Bool("compensation.logic", true),
	)

	log.Printf("Stock release successful for item %s, order %s", itemID, orderID)
	span.AddEvent("Stock released")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Stock released"))
}

// checkStockHandler 保持不变，可以作为普通查询接口
func checkStockHandler(w http.ResponseWriter, r *http.Request) {
	propagator := otel.GetTextMapPropagator()
	ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
	_, span := tracer.Start(ctx, "inventory-service.CheckStock")
	defer span.End()

	itemID := r.URL.Query().Get("itemID")
	log.Printf("Stock check successful for item %s", itemID)
	span.AddEvent("Stock check successful")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Stock available"))
}
