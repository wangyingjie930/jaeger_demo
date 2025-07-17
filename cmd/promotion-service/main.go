// cmd/promotion-service/main.go
package main

import (
	"encoding/json"
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
			ctx.Mux.HandleFunc("/use_coupon", handleUseCoupon)
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

// UseCouponRequest 定义了核销优惠券的请求结构
type UseCouponRequest struct {
	UserID      string   `json:"user_id"`
	CouponCode  string   `json:"coupon_code"`
	OrderID     string   `json:"order_id"`
	OrderAmount float64  `json:"order_amount"`
	ItemIDs     []string `json:"item_ids"`
}

// UseCouponResponse 定义了核销优惠券的响应结构
type UseCouponResponse struct {
	Success        bool    `json:"success"`
	DiscountAmount float64 `json:"discount_amount"`
	FinalAmount    float64 `json:"final_amount"`
	Message        string  `json:"message"`
}

// handleUseCoupon 是核销优惠券的核心逻辑
func handleUseCoupon(w http.ResponseWriter, r *http.Request) {
	propagator := otel.GetTextMapPropagator()
	ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
	ctx, span := tracer.Start(ctx, "promotion-service.UseCoupon")
	defer span.End()

	var req UseCouponRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	span.SetAttributes(
		attribute.String("user.id", req.UserID),
		attribute.String("coupon.code", req.CouponCode),
		attribute.String("order.id", req.OrderID),
	)

	// 在这里实现完整的核销逻辑:
	// 1. 检查 coupon_code 是否存在、是否属于该用户、状态是否为“未使用”。
	// 2. 根据 template_id 查询优惠券模板信息。
	// 3. 校验适用范围 (scope_type, scope_value) 和门槛 (threshold_amount)。
	// 4. 如果校验通过:
	//    a. 计算优惠金额。
	//    b. **【核心】将 user_coupon 表中的状态更新为 "已冻结(FROZEN)"**, 并记录 order_id。
	//    c. 返回计算后的价格。
	// 5. 如果校验失败，返回错误信息。

	// 模拟核销成功
	log.Printf("Coupon %s for order %s has been used (frozen).", req.CouponCode, req.OrderID)

	// 模拟计算
	discount := 10.0
	finalAmount := req.OrderAmount - discount

	resp := UseCouponResponse{
		Success:        true,
		DiscountAmount: discount,
		FinalAmount:    finalAmount,
		Message:        "Coupon applied successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
