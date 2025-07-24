// promotion-service/internal/domain/promotion_template.go
package domain

import "time"

// DiscountType 定义了优惠的类型，它将用于策略工厂决定使用哪种DiscountStrategy。
type DiscountType string

const (
	DiscountTypeFixedAmount DiscountType = "FIXED_AMOUNT" // 满减/立减
	DiscountTypePercentage  DiscountType = "PERCENTAGE"   // 折扣
	// 未来可以轻松扩展, e.g., DiscountTypeFreebie, DiscountTypePoints
)

// PromotionTemplate 是优惠的核心定义，它是一个不可变对象。
// 任何对模板的修改都应该创建一个新的版本，而不是在原地更新。
type PromotionTemplate struct {
	ID          int64
	Version     int32  // 版本号，每次编辑时递增
	Name        string // e.g., "双十一超级满减券"
	Description string // 详细描述
	Status      string // e.g., "ACTIVE", "INACTIVE", "ARCHIVED"
	CreatedAt   time.Time
	UpdatedAt   time.Time

	// --- 规则与策略 ---
	// RuleDefinition 是一个JSON字符串，定义了此优惠的适用条件 (LHS)。
	// 它将被传递给RuleEngine进行评估。
	RuleDefinition string

	// DiscountType 标识了优惠的计算方式 (RHS)。
	// 它将用于策略工厂来获取正确的DiscountStrategy。
	DiscountType DiscountType

	// DiscountProperties 是一个JSON字符串，存储了具体策略所需的参数。
	// 例如，对于满减券是 {"threshold": 20000, "amount": 2000}
	// 对于折扣券是 {"percentage": 88, "ceiling": 5000} (88折，最多优惠50元)
	DiscountProperties string
}
