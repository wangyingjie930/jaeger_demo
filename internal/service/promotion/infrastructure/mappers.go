package infrastructure

import (
	"nexus/internal/service/promotion/domain"
)

// --- PromotionTemplate Mappers ---

func toDomainPromotionTemplate(model *PromotionTemplateModel) *domain.PromotionTemplate {
	if model == nil {
		return nil
	}
	return &domain.PromotionTemplate{
		ID:                 model.ID,
		TemplateGroupID:    model.TemplateGroupID,
		Version:            model.Version,
		Name:               model.Name,
		Description:        model.Description,
		PromotionType:      model.PromotionType,
		RuleDefinition:     model.RuleDefinition,
		DiscountType:       domain.DiscountType(model.DiscountType),
		DiscountProperties: model.DiscountProperties,
		StartDate:          model.StartDate,
		EndDate:            model.EndDate,
		IsExclusive:        model.IsExclusive,
		Priority:           model.Priority,
		IsActive:           model.IsActive,
		CreatedAt:          model.CreatedAt,
		UpdatedAt:          model.UpdatedAt,
	}
}

func toGormPromotionTemplate(domain *domain.PromotionTemplate) *PromotionTemplateModel {
	if domain == nil {
		return nil
	}
	return &PromotionTemplateModel{
		ID:                 domain.ID,
		TemplateGroupID:    domain.TemplateGroupID,
		Version:            domain.Version,
		Name:               domain.Name,
		Description:        domain.Description,
		PromotionType:      domain.PromotionType,
		RuleDefinition:     domain.RuleDefinition,
		DiscountType:       string(domain.DiscountType),
		DiscountProperties: domain.DiscountProperties,
		StartDate:          domain.StartDate,
		EndDate:            domain.EndDate,
		IsExclusive:        domain.IsExclusive,
		Priority:           domain.Priority,
		IsActive:           domain.IsActive,
		CreatedAt:          domain.CreatedAt,
		UpdatedAt:          domain.UpdatedAt,
	}
}

// --- UserCoupon Mappers ---

func toDomainUserCoupon(model *UserCouponModel) *domain.UserCoupon {
	if model == nil {
		return nil
	}
	return &domain.UserCoupon{
		ID:         model.ID,
		UserID:     model.UserID,
		CouponCode: model.CouponCode,
		TemplateID: model.TemplateID,
		Status:     model.Status,
		IssueDate:  model.IssueDate,
		ExpiryDate: model.ExpiryDate,
		UsedAt:     model.UsedAt,
		CreatedAt:  model.CreatedAt,
		UpdatedAt:  model.UpdatedAt,
	}
}

func toGormUserCoupon(domain *domain.UserCoupon) *UserCouponModel {
	if domain == nil {
		return nil
	}
	return &UserCouponModel{
		ID:         domain.ID,
		UserID:     domain.UserID,
		CouponCode: domain.CouponCode,
		TemplateID: domain.TemplateID,
		Status:     domain.Status,
		IssueDate:  domain.IssueDate,
		ExpiryDate: domain.ExpiryDate,
		UsedAt:     domain.UsedAt,
		CreatedAt:  domain.CreatedAt,
		UpdatedAt:  domain.UpdatedAt,
	}
}
