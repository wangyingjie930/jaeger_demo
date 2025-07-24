// promotion-service/internal/domain/discount.go
package domain

// DiscountApplication 表示一次优惠计算的具体结果。
type DiscountApplication struct {
	Amount       int64  // 优惠的总金额
	StrategyName string // 应用的策略名称，用于追踪和调试
	Description  string // 优惠的描述，可用于向用户展示
}

// DiscountStrategy 定义了优惠计算策略的接口。
// 它的职责是：根据给定的事实（Fact）和优惠券模板信息，计算出具体的优惠金额。
type DiscountStrategy interface {
	// Calculate 计算优惠
	// fact: 包含所有上下文信息的事实对象
	// template: 优惠券模板，策略可能需要模板中的一些元数据（如折扣率、满减金额）
	// 返回值: *DiscountApplication 描述了本次优惠计算的结果, error 计算过程中的错误
	Calculate(fact Fact, template *PromotionTemplate) (*DiscountApplication, error)
}
