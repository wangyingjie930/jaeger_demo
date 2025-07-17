package port

import (
	"encoding/json"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"net/http"
	"nexus/internal/service/promotion/application"
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
		// 根据错误类型返回不同的 HTTP 状态码
		// 例如 domain.ErrCouponNotFound -> http.StatusNotFound
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
