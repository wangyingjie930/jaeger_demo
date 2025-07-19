// internal/service/order/domain/event.go
package domain

import "time"

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
