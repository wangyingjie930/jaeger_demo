package main

import (
	"encoding/json"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"jaeger-demo/internal/pkg/bootstrap"
	"jaeger-demo/internal/pkg/mq"
	"jaeger-demo/internal/service/order"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const (
	serviceName        = "api-gateway"
	orderCreationTopic = "order-creation-topic" // ✨ 新增：订单创建主题
)

// <<<<<<< 新增特性开关 >>>>>>>>>
var featureFlags = map[string]bool{
	"PROMO_VIP_SUMMER_2025": true, // 硬编码一个特性开关
}

// <<<<<<< 新增特性开关 >>>>>>>>>

func main() {
	bootstrap.Init()

	// ✨ 核心改造：初始化一个全局的 Kafka Writer
	kafkaWriter := mq.NewKafkaWriter(strings.Split(bootstrap.GetCurrentConfig().Infra.Kafka.Brokers, ","), orderCreationTopic)
	defer kafkaWriter.Close()

	bootstrap.StartService(bootstrap.AppInfo{
		ServiceName: "api-gateway",
		Port:        8080,
		RegisterHandlers: func(appCtx bootstrap.AppCtx) {
			appCtx.Mux.Handle("/metrics", promhttp.Handler())
			// <<<<<<< 新增: 注册健康检查路由 >>>>>>>>>
			appCtx.Mux.HandleFunc("/healthz", healthzHandler)
			// readinessProbe可以简化，因为不再直接依赖order-service的HTTP接口
			appCtx.Mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
				// 在异步模型中，网关的就绪状态主要取决于自身和到MQ的连接
				// 这里可以添加检查Kafka连接状态的逻辑
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			})

			appCtx.Mux.HandleFunc("/create_complex_order", func(w http.ResponseWriter, r *http.Request) {
				createOrderHandler(w, r, kafkaWriter)
			})
		},
	})
}

func createOrderHandler(w http.ResponseWriter, r *http.Request, writer *kafka.Writer) {
	tracer := otel.Tracer(serviceName)
	// 初始上下文
	ctx := r.Context()

	isVIP := r.URL.Query().Get("is_vip") == "true"
	isPromotionActive := featureFlags["PROMO_VIP_SUMMER_2025"]

	ctx, span := tracer.Start(ctx, "api-gateway.ComplexOrderHandler")
	defer span.End()

	span.SetAttributes(
		attribute.Bool("user.is_vip", isVIP),
		attribute.Bool("feature_flag.PROMO_VIP_SUMMER_2025", isPromotionActive),
		attribute.String("messaging.system", "kafka"),
		attribute.String("messaging.destination", orderCreationTopic),
	)

	// 如果是VIP用户且活动开启，则通过Baggage向下游传递业务信息
	var promoId string
	if isVIP && isPromotionActive {
		promoId = "VIP_SUMMER_SALE"
		log.Println("VIP user detected, activating promotion baggage.")
		promoBaggage, _ := baggage.NewMember("promotion_id", promoId)
		b, _ := baggage.FromContext(ctx).SetMember(promoBaggage)
		ctx = baggage.ContextWithBaggage(ctx, b)
		span.AddEvent("Baggage with promotion_id injected.")
	}

	// 2. ✨ 核心改造：构建异步消息，而不是HTTP请求
	quantity, _ := strconv.Atoi(r.URL.Query().Get("quantity"))
	if quantity == 0 {
		quantity = 1 // 默认数量
	}

	event := order.OrderCreationEvent{
		TraceID:          span.SpanContext().TraceID().String(),
		UserID:           r.URL.Query().Get("userId"),
		IsVIP:            isVIP,
		Items:            strings.Split(r.URL.Query().Get("items"), ","),
		Quantity:         quantity,
		PromoId:          promoId,
		SeckillProductID: r.URL.Query().Get("seckill_product_id"),
		EventId:          uuid.New().String(),
	}

	eventBytes, err := json.Marshal(event)
	if err != nil {
		log.Printf("ERROR: Failed to marshal order creation event: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// 3. ✨ 核心改造：使用通用方法发送消息到Kafka
	// 这个方法会自动注入当前的Trace Context到消息头中
	err = mq.ProduceMessage(ctx, writer, []byte(event.UserID), eventBytes)
	if err != nil {
		log.Printf("ERROR: Failed to produce message to Kafka: %v", err)
		span.RecordError(err)
		http.Error(w, "Failed to submit order request, please try again later.", http.StatusServiceUnavailable)
		return
	}

	span.AddEvent("Order creation request successfully sent to Kafka.")
	log.Printf("Successfully sent order creation request for user %s to topic %s", event.UserID, orderCreationTopic)

	// 4. ✨ 核心改造：立即返回，给用户良好体验
	w.WriteHeader(http.StatusAccepted) // 202 Accepted 是一个非常适合此场景的状态码
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "pending",
		"message":  "Your order is being processed. You will receive a notification upon completion.",
		"event_id": event.EventId,
	})
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// <<<<<<< 新增: 健康检查 Handler >>>>>>>>>
func healthzHandler(w http.ResponseWriter, r *http.Request) {
	// 对于 livenessProbe，只要能响应，就说明进程存活
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
