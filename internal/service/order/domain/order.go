// internal/service/order/domain/order.go
package domain

import (
	"errors"
	"time"
)

// Order 是订单聚合的根实体
type Order struct {
	ID        string
	UserID    string
	IsVIP     bool
	Items     []string // 在简单场景下可以是[]string, 复杂场景下是[]OrderItem值对象
	Quantity  int
	PromoID   string
	State     State
	CreatedAt time.Time
	UpdatedAt time.Time

	SeckillProductID string

	// 价格、折扣等信息也可以是领域的一部分
	// FinalAmount float64
}

// 工厂函数: NewOrder 用于创建一个新的订单实例
func NewOrder(event *OrderCreationRequested) (*Order, error) {
	if event.EventID == "" || event.UserID == "" || len(event.Items) == 0 {
		return nil, errors.New("cannot create order with empty required fields")
	}

	return &Order{
		ID:               event.EventID,
		UserID:           event.UserID,
		IsVIP:            event.IsVIP,
		Items:            event.Items,
		Quantity:         event.Quantity,
		PromoID:          event.PromoID,
		State:            StateCreated, // 初始状态
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		SeckillProductID: event.SeckillProductID,
	}, nil
}

// MarkAsPendingPayment 将订单状态更新为等待支付
// 这个方法只负责状态流转，不负责调用外部服务
func (o *Order) MarkAsPendingPayment() error {
	if o.State != StateCreated && o.State != StateValidation {
		return errors.New("order can only be marked as pending payment from created or validating state")
	}
	o.State = StatePendingPayment
	o.UpdatedAt = time.Now()
	return nil
}

// MarkAsFailed 将订单标记为失败
func (o *Order) MarkAsFailed() {
	o.State = StateFailed
	o.UpdatedAt = time.Now()
}

// Cancel 取消订单
func (o *Order) Cancel() error {
	// 只有待支付的订单可以被取消
	if o.State != StatePendingPayment {
		return errors.New("only pending payment orders can be cancelled")
	}
	o.State = StateCancelled
	o.UpdatedAt = time.Now()
	return nil
}

// Pay 支付订单 (示例)
func (o *Order) Pay() error {
	if o.State != StatePendingPayment {
		return errors.New("only pending payment orders can be paid")
	}
	o.State = StatePaid
	o.UpdatedAt = time.Now()
	return nil
}
