// internal/service/order/events.go
package order

// OrderCreationEvent 是从API网关发送到Kafka的订单创建请求消息
type OrderCreationEvent struct {
	TraceID          string   `json:"traceId"`
	UserID           string   `json:"userId"`
	IsVIP            bool     `json:"isVip"`
	Items            []string `json:"items"`
	Quantity         int      `json:"quantity"`
	PromoId          string   `json:"promoId,omitempty"`          // 从Baggage中提取的促销活动ID
	SeckillProductID string   `json:"seckillProductId,omitempty"` // 秒杀商品ID
	EventId          string   `json:"eventId"`
}

// NotificationEvent 定义了要发送到 Kafka 的消息结构
type NotificationEvent struct {
	UserID      string `json:"userId"`
	Message     string `json:"message"`
	PromotionID string `json:"promotion_id,omitempty"`
}
