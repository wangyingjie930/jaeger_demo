package infrastructure

import (
	"gorm.io/gorm"
	"nexus/internal/service/promotion/domain"
	"strings"
)

// ToDomainUserCoupon 将数据库模型转换为领域模型
func ToDomainUserCoupon(model *UserCouponModel) *domain.UserCoupon {
	if model == nil {
		return nil
	}
	return &domain.UserCoupon{
		ID:         int64(model.ID),
		CouponCode: model.CouponCode,
		UserID:     model.UserID,
		Status:     model.Status,
		ValidTo:    model.ValidTo,
		Template:   ToDomainCouponTemplate(&model.Template),
	}
}

// ToDomainCouponTemplate 将数据库模型转换为领域模型
func ToDomainCouponTemplate(model *CouponTemplateModel) *domain.CouponTemplate {
	if model == nil {
		return nil
	}
	return &domain.CouponTemplate{
		ID:              int64(model.ID),
		TemplateCode:    model.TemplateCode,
		Name:            model.Name,
		Type:            model.Type,
		DiscountValue:   model.DiscountValue,
		ThresholdAmount: model.ThresholdAmount,
		ScopeType:       model.ScopeType,
		ScopeValue:      strings.Split(model.ScopeValue, ","), // 将字符串转换为切片
	}
}

// FromDomainUserCoupon 将领域模型转换为数据库模型 (用于更新)
// 注意：这里我们只转换需要更新的字段，或者创建一个完整的模型用于插入
func FromDomainUserCoupon(dmn *domain.UserCoupon) *UserCouponModel {
	if dmn == nil {
		return nil
	}
	return &UserCouponModel{
		Model: gorm.Model{
			ID: uint(dmn.ID),
		},
		CouponCode: dmn.CouponCode,
		UserID:     dmn.UserID,
		Status:     dmn.Status,
		ValidTo:    dmn.ValidTo,
		TemplateID: uint(dmn.Template.ID),
	}
}
