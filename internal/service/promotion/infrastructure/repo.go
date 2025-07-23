// promotion-service/internal/infrastructure/persistence/promotion_repo.go
package persistence

import (
	"gorm.io/gorm"
	"nexus/internal/service/promotion/domain"
)

// GormPromotionRepository 是促销相关实体的仓储实现。
type GormPromotionRepository struct {
	db *gorm.DB
}

func NewGormPromotionRepository(db *gorm.DB) *GormPromotionRepository {
	return &GormPromotionRepository{db: db}
}

// FindTemplateByIDAndVersion 根据ID和版本查找优惠模板。
func (r *GormPromotionRepository) FindTemplateByIDAndVersion(id int64, version int32) (*domain.PromotionTemplate, error) {
	var model PromotionTemplateModel
	if err := r.db.Where("id = ? AND version = ?", id, version).First(&model).Error; err != nil {
		return nil, err
	}
	return toDomainTemplate(&model), nil
}

// FindUserCouponsByUserID 查找某个用户所有可用的优惠券。
func (r *GormPromotionRepository) FindUserCouponsByUserID(userID int64) ([]*domain.UserCoupon, error) {
	var models []*UserCouponModel
	// 实际业务中可能还需要加上更多过滤条件，比如状态为 UNUSED 且未过期
	if err := r.db.Where("user_id = ?", userID).Find(&models).Error; err != nil {
		return nil, err
	}

	coupons := make([]*domain.UserCoupon, len(models))
	for i, m := range models {
		coupons[i] = toDomainCoupon(m)
	}
	return coupons, nil
}

// UpdateUserCouponStatus 更新用户优惠券的状态。
func (r *GormPromotionRepository) UpdateUserCouponStatus(id int64, status domain.UserCouponStatus) error {
	return r.db.Model(&UserCouponModel{}).Where("id = ?", id).Update("status", status).Error
}

// --- 类型转换函数 ---
// 将数据库模型转换为领域模型
func toDomainTemplate(model *PromotionTemplateModel) *domain.PromotionTemplate {
	return &domain.PromotionTemplate{
		ID:                 model.ID,
		Version:            model.Version,
		Name:               model.Name,
		Description:        model.Description,
		Status:             model.Status,
		RuleDefinition:     model.RuleDefinition,
		DiscountType:       domain.DiscountType(model.DiscountType),
		DiscountProperties: model.DiscountProperties,
		CreatedAt:          model.CreatedAt,
		UpdatedAt:          model.UpdatedAt,
	}
}

func toDomainCoupon(model *UserCouponModel) *domain.UserCoupon {
	return &domain.UserCoupon{
		ID:              model.ID,
		UserID:          model.UserID,
		Status:          domain.UserCouponStatus(model.Status),
		ReceivedAt:      model.ReceivedAt,
		UsedAt:          model.UsedAt,
		ExpiredAt:       model.ExpiredAt,
		TemplateID:      model.TemplateID,
		TemplateVersion: model.TemplateVersion,
	}
}
