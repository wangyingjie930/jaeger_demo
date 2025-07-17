package infrastructure

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"nexus/internal/service/promotion/domain"
)

// GormCouponRepository 是 CouponRepository 的 GORM 实现
type GormCouponRepository struct {
	db *gorm.DB
}

// NewGormCouponRepository 创建一个新的 GORM 仓储实例
func NewGormCouponRepository(db *gorm.DB) *GormCouponRepository {
	return &GormCouponRepository{db: db}
}

// FindByCode 使用 GORM 从数据库中查找优惠券
func (r *GormCouponRepository) FindByCode(ctx context.Context, code string) (*domain.UserCoupon, error) {
	var model UserCouponModel
	// 使用 Preload 来预加载关联的模板信息
	err := r.db.WithContext(ctx).Preload("Template").Where("coupon_code = ?", code).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrCouponNotFound
		}
		return nil, err
	}
	// 使用 Mapper 将数据库模型转换为领域模型
	return ToDomainUserCoupon(&model), nil
}

// Save 使用 GORM 将优惠券状态变更保存回数据库
func (r *GormCouponRepository) Save(ctx context.Context, coupon *domain.UserCoupon) error {
	// 在真实场景中，我们可能只需要更新变化的字段
	// 这里为了演示，我们使用 map 来指定更新的字段
	updateData := map[string]interface{}{
		"status": coupon.Status,
		// "order_id": coupon.OrderID, // 假设 OrderID 在领域模型中
	}
	// GORM 的 .Model(&UserCouponModel{}) 指定了要操作的表
	// .Where() 提供了查询条件
	// .Updates() 执行部分更新
	err := r.db.WithContext(ctx).Model(&UserCouponModel{}).Where("id = ?", coupon.ID).Updates(updateData).Error
	return err
}
