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
// 这个模型现在包含了所有业务逻辑和规则评估所需的完整字段。
type PromotionTemplate struct {
	ID              int64
	TemplateGroupID string // [新增] 模板组ID，用于标识同一促销活动的不同版本 [cite: 175]
	Version         int32  // 版本号，每次编辑时递增 [cite: 186]
	Name            string // e.g., "双十一超级满减券"
	Description     string // 详细描述
	PromotionType   string // [新增] 促销类型, 如 'STORE_COUPON', 'PLATFORM_SALE' [cite: 179]

	// --- 核心规则与策略字段 ---
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

	// --- 生命周期与元数据 ---
	StartDate   time.Time // [新增] 活动生效时间 [cite: 182]
	EndDate     time.Time // [新增] 活动失效时间 [cite: 182]
	IsExclusive bool      // [新增] 是否与其它优惠互斥 [cite: 183]
	Priority    int       // [新增] 优先级, 数字越大优先级越高 [cite: 184]
	IsActive    bool      // [新增] 当前版本是否激活 [cite: 185]

	// --- 时间戳 ---
	CreatedAt time.Time
	UpdatedAt time.Time
}

// IsAvailable 检查模板在当前时间是否有效。
func (pt *PromotionTemplate) IsAvailable() bool {
	now := time.Now()
	return pt.IsActive && !now.Before(pt.StartDate) && !now.After(pt.EndDate)
}
