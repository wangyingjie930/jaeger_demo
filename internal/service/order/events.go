// internal/service/order/events.go
package order

import "time"

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

// ✨ 新增: OrderTimeoutCheckEvent 定义了支付超时检查的任务消息
type OrderTimeoutCheckEvent struct {
	TraceID string   `json:"traceId"`
	OrderID string   `json:"orderId"`
	UserID  string   `json:"userId"`
	Items   []string `json:"items"`
	// 为了方便追踪和调试，可以加上事件创建的时间
	CreationTime time.Time `json:"creationTime"`
}
