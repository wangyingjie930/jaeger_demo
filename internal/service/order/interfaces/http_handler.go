package interfaces

import (
	"encoding/json"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/wangyingjie930/nexus-pkg/bootstrap"
	"github.com/wangyingjie930/nexus-pkg/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/propagation"
	"net/http"
	"nexus/internal/service/order/application"
	"strconv"
	"strings"
)

const (
	serviceName        = "order-service"
	orderCreationTopic = "order-creation-topic"
)

// OrderHandler 封装了 promotion 服务的 HTTP 处理器
type OrderHandler struct {
	service *application.OrderApplicationService
}

// NewOrderHandler 创建一个新的 HTTP 处理器实例
func NewOrderHandler(service *application.OrderApplicationService) *OrderHandler {
	return &OrderHandler{service: service}
}

// RegisterRoutes 在 ServeMux 上注册所有路由
func (h *OrderHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/create_complex_order", h.createOrderHandler)
}

func (h *OrderHandler) createOrderHandler(w http.ResponseWriter, r *http.Request) {
	propagator := otel.GetTextMapPropagator()
	ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

	isVIP := r.URL.Query().Get("is_vip") == "true"

	tracer := otel.Tracer(serviceName)
	ctx, span := tracer.Start(ctx, "api-gateway.ComplexOrderHandler")
	defer span.End()

	quantity, _ := strconv.Atoi(r.URL.Query().Get("quantity"))
	if quantity == 0 {
		quantity = 1 // 默认数量
	}

	span.SetAttributes(
		attribute.Bool("user.is_vip", isVIP),
		attribute.Bool("feature_flag.PROMO_VIP_SUMMER_2025", bootstrap.GetCurrentConfig().App.FeatureFlags.EnableVipPromotion),
		attribute.String("messaging.system", "kafka"),
		attribute.String("messaging.destination", orderCreationTopic),
	)

	// 如果是VIP用户且活动开启，则通过Baggage向下游传递业务信息
	var promoId string
	if isVIP && bootstrap.GetCurrentConfig().App.FeatureFlags.EnableVipPromotion {
		promoId = "VIP_SUMMER_SALE"
		logger.Ctx(ctx).Info().Msg("VIP user detected, activating promotion baggage.")
		promoBaggage, _ := baggage.NewMember("promotion_id", promoId)
		b, _ := baggage.FromContext(ctx).SetMember(promoBaggage)
		ctx = baggage.ContextWithBaggage(ctx, b)
		span.AddEvent("Baggage with promotion_id injected.")
	}

	resp, err := h.service.RequestOrderCreation(ctx, &application.CreateOrderRequest{
		UserID:           r.URL.Query().Get("userId"),
		IsVIP:            isVIP,
		Items:            strings.Split(r.URL.Query().Get("items"), ","),
		Quantity:         quantity,
		PromoID:          promoId,
		SeckillProductID: r.URL.Query().Get("seckill_product_id"),
	})

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
