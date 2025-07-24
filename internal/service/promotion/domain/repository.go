package domain

import "context"

// CouponRepository 定义了优惠券数据的持久化接口
// 这是领域层与基础设施层之间的“插座”
type CouponRepository interface {
	FindByCode(ctx context.Context, code string) (*UserCoupon, error)
	FindByID(ctx context.Context, id int64) (*UserCoupon, error)
	FindByUserID(ctx context.Context, userID int64) ([]*UserCoupon, error)
	Save(ctx context.Context, coupon *UserCoupon) error
	Update(ctx context.Context, coupon *UserCoupon) error
}

// PromotionTemplateRepository 定义了促销模板的持久化接口
type PromotionTemplateRepository interface {
	// FindByID 获取指定ID和版本的模板
	FindByID(ctx context.Context, id int64) (*PromotionTemplate, error)
	// FindByGroupIDAndVersion 获取指定组ID和版本的模板
	FindByGroupIDAndVersion(ctx context.Context, groupID string, version int32) (*PromotionTemplate, error)
	// FindLatestByGroupID 获取一个模板组的最新版本
	FindLatestByGroupID(ctx context.Context, groupID string) (*PromotionTemplate, error)
	// FindActiveByGroupID 获取一个模板组当前激活的版本
	FindActiveByGroupID(ctx context.Context, groupID string) (*PromotionTemplate, error)
	// FindAllActiveTemplates 获取所有激活的模板，用于后续筛选
	FindAllActiveTemplates(ctx context.Context) ([]*PromotionTemplate, error)
	// Create 创建一个新的模板
	Create(ctx context.Context, template *PromotionTemplate) error
	// Update 更新一个模板 (通常是状态)
	Update(ctx context.Context, template *PromotionTemplate) error
}
