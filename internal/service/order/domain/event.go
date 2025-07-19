// internal/service/order/domain/event.go
package domain

import (
	"context"
	"time"
)

// OrderCreationRequested 是当用户请求创建一个新订单时发布的事件
// 注意：这更像一个命令的载体，但在异步流程中，我们将其视为一个触发事件。
type OrderCreationRequested struct {
	TraceID          string   `json:"traceId"`
	UserID           string   `json:"userId"`
	IsVIP            bool     `json:"isVip"`
	Items            []string `json:"items"`
	Quantity         int      `json:"quantity"`
	PromoID          string   `json:"promoId,omitempty"`
	SeckillProductID string   `json:"seckillProductId,omitempty"`
	EventID          string   `json:"eventId"`
}

// OrderSuccessfullyPlaced 是当订单成功创建并等待支付时发布的事件
type OrderSuccessfullyPlaced struct {
	OrderID      string
	UserID       string
	TotalAmount  float64 // 假设应用层计算了总价
	PlacedAt     time.Time
	PaymentDueBy time.Time
}

// OrderCreationFailed 是当订单因任何原因创建失败时发布的事件
type OrderCreationFailed struct {
	OrderID string
	UserID  string
	Reason  string
	At      time.Time
}

// ... 其他领域事件, e.g., OrderPaid, OrderCancelled

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

type OrderProducer interface {
	Product(ctx context.Context, event *OrderCreationRequested) error
}
