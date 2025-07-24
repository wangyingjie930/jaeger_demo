package infrastructure

import (
	"nexus/internal/service/promotion/domain"
	"time"
)

// PromotionTemplateModel 对应于数据库中的 `promotion_templates` 表
// 它存储了促销活动的通用定义，是可复用的模板。
type PromotionTemplateModel struct {
	ID              int64  `gorm:"primaryKey"`
	TemplateGroupID string `gorm:"type:varchar(100);index;comment:模板组ID，用于标识同一促销活动的不同版本"` // 标识同一活动的所有版本
	Version         int32  `gorm:"not null;default:1;comment:版本号"`                        // 版本号，每次编辑时递增
	Name            string `gorm:"type:varchar(255);not null;comment:促销名称, 如 '双十一跨店满减'"`
	Description     string `gorm:"type:text;comment:详细描述"`
	PromotionType   string `gorm:"type:varchar(50);not null;comment:促销类型, 如 'STORE_COUPON', 'PLATFORM_SALE'"`

	// --- 核心规则与策略字段 ---
	RuleDefinition     string `gorm:"type:json;comment:规则定义(LHS), 存储JSON格式的适用条件"`                                  // [cite: 199]
	DiscountType       string `gorm:"type:varchar(50);not null;comment:优惠类型(RHS), 如 'FIXED_AMOUNT', 'PERCENTAGE'"` // [cite: 189]
	DiscountProperties string `gorm:"type:json;comment:优惠策略需要的参数, 如满减门槛、折扣率等"`

	// --- 生命周期与元数据 ---
	StartDate   time.Time `gorm:"comment:活动生效时间"`
	EndDate     time.Time `gorm:"comment:活动失效时间"`
	IsExclusive bool      `gorm:"default:true;comment:是否与其它优惠互斥"`   // [cite: 192]
	Priority    int       `gorm:"default:0;comment:优先级, 数字越大优先级越高"` // [cite: 193]
	IsActive    bool      `gorm:"default:true;comment:当前版本是否激活"`    // [cite: 194]

	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

// UserCouponModel 对应于数据库中的 `user_coupons` 表
// 代表一个用户持有的一张具体的优惠券实例。
type UserCouponModel struct {
	ID         int64  `gorm:"primaryKey"`
	UserID     int64  `gorm:"not null;index;comment:所属用户ID"`
	CouponCode string `gorm:"type:varchar(100);uniqueIndex;comment:券码, 用于核销"`

	// --- 关键关联 ---
	TemplateID int64 `gorm:"not null;index;comment:关联到促销模板表的ID"` // 直接关联到某个版本的模板

	// --- 状态与生命周期 ---
	Status     domain.UserCouponStatus `gorm:"type:varchar(20);not null;index;comment:状态 (UNUSED, FROZEN, USED, EXPIRED)"`
	IssueDate  time.Time               `gorm:"autoCreateTime;comment:发放日期"`
	ExpiryDate time.Time               `gorm:"comment:失效日期"`
	UsedAt     *time.Time              `gorm:"comment:使用时间"` // 使用指针类型，允许为NULL

	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}
