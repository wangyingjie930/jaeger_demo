// internal/service/order/domain/state.go
package domain

// State 定义了订单的生命周期状态
type State string

const (
	StateCreated        State = "CREATED"         // 订单已在系统中记录，但未经验证
	StateValidation     State = "VALIDATING"      // 订单正在验证中（例如，库存、欺诈检测）
	StatePendingPayment State = "PENDING_PAYMENT" // 验证通过，等待用户支付
	StatePaid           State = "PAID"            // 已支付
	StateCancelled      State = "CANCELLED"       // 已取消 (用户主动或系统超时)
	StateFailed         State = "FAILED"          // 订单处理失败
)
