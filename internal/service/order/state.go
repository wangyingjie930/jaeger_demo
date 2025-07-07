// internal/service/order/state.go
package order

type OrderState string

const (
	StateCreated        OrderState = "CREATED"         // 订单已创建（中间状态）
	StatePendingPayment OrderState = "PENDING_PAYMENT" // 待支付
	StatePaid           OrderState = "PAID"            // 已支付
	StateCancelled      OrderState = "CANCELLED"       // 已取消 (用户或系统)
	StateFailed         OrderState = "FAILED"          // 创建失败
)
