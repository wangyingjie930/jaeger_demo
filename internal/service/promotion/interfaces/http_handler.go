package interfaces

import (
	"encoding/json"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"net/http"
	"nexus/internal/service/promotion/application"
	"nexus/internal/service/promotion/domain"
)

// PromotionHandler 封装了 promotion 服务的 HTTP 处理器
type PromotionHandler struct {
	service *application.PromotionService
}

// NewPromotionHandler 创建一个新的 HTTP 处理器实例
func NewPromotionHandler(service *application.PromotionService) *PromotionHandler {
	return &PromotionHandler{service: service}
}

// RegisterRoutes 在 ServeMux 上注册所有路由
func (h *PromotionHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/get_promo_price", h.handleGetPromoPrice)
	mux.HandleFunc("/use_coupon", h.handleUseCoupon)
	mux.HandleFunc("/cancel_coupon", h.handleCancelCoupon)
}

func (h *PromotionHandler) handleGetPromoPrice(w http.ResponseWriter, r *http.Request) {
	propagator := otel.GetTextMapPropagator()
	ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

	isVip := r.URL.Query().Get("is_vip") == "true"
	userID := r.URL.Query().Get("user_id")

	resp, err := h.service.GetPromoPrice(ctx, isVip, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *PromotionHandler) handleUseCoupon(w http.ResponseWriter, r *http.Request) {
	propagator := otel.GetTextMapPropagator()
	ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

	var req application.UseCouponRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	resp, err := h.service.UseCoupon(ctx, &req)
	if err != nil {
		// ✨ [核心改造] 根据错误类型返回不同的 HTTP 状态码
		var statusCode int
		switch {
		case errors.Is(err, domain.ErrCouponNotFound):
			statusCode = http.StatusNotFound
		case errors.Is(err, domain.ErrCouponExpired),
			errors.Is(err, domain.ErrCouponAlreadyUsed),
			errors.Is(err, domain.ErrCouponNotApplicable),
			errors.Is(err, domain.ErrCouponStatusInvalid):
			statusCode = http.StatusForbidden // 客户端请求有效，但服务器拒绝执行
		default:
			statusCode = http.StatusInternalServerError // 其他未知错误
		}
		http.Error(w, err.Error(), statusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleCancelCoupon 是补偿接口的处理器
func (h *PromotionHandler) handleCancelCoupon(w http.ResponseWriter, r *http.Request) {
	propagator := otel.GetTextMapPropagator()
	ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

	var req application.UseCouponRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := h.service.CancelCouponUsage(ctx, &req)
	if err != nil {
		// 补偿操作失败是一个严重问题，应该返回 500
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Coupon usage successfully cancelled.",
	})
}
