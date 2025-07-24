package infrastructure

import (
	"context"
	"gorm.io/gorm"
	"nexus/internal/service/promotion/domain"
)

type gormPromotionTemplateRepository struct {
	db *gorm.DB
}

func NewGormPromotionTemplateRepository(db *gorm.DB) domain.PromotionTemplateRepository {
	return &gormPromotionTemplateRepository{db: db}
}

func (r *gormPromotionTemplateRepository) FindByID(ctx context.Context, id int64) (*domain.PromotionTemplate, error) {
	var model PromotionTemplateModel
	if err := r.db.WithContext(ctx).First(&model, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // Or a specific domain error
		}
		return nil, err
	}
	return toDomainPromotionTemplate(&model), nil
}

func (r *gormPromotionTemplateRepository) FindByGroupIDAndVersion(ctx context.Context, groupID string, version int32) (*domain.PromotionTemplate, error) {
	var model PromotionTemplateModel
	if err := r.db.WithContext(ctx).Where("template_group_id = ? AND version = ?", groupID, version).First(&model).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return toDomainPromotionTemplate(&model), nil
}

func (r *gormPromotionTemplateRepository) FindLatestByGroupID(ctx context.Context, groupID string) (*domain.PromotionTemplate, error) {
	var model PromotionTemplateModel
	if err := r.db.WithContext(ctx).Where("template_group_id = ?", groupID).Order("version desc").First(&model).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return toDomainPromotionTemplate(&model), nil
}

func (r *gormPromotionTemplateRepository) FindActiveByGroupID(ctx context.Context, groupID string) (*domain.PromotionTemplate, error) {
	var model PromotionTemplateModel
	if err := r.db.WithContext(ctx).Where("template_group_id = ? AND is_active = ?", groupID, true).First(&model).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return toDomainPromotionTemplate(&model), nil
}

func (r *gormPromotionTemplateRepository) FindAllActiveTemplates(ctx context.Context) ([]*domain.PromotionTemplate, error) {
	var models []*PromotionTemplateModel
	if err := r.db.WithContext(ctx).Where("is_active = ? AND start_date <= NOW() AND end_date >= NOW()", true).Find(&models).Error; err != nil {
		return nil, err
	}

	var templates []*domain.PromotionTemplate
	for _, model := range models {
		templates = append(templates, toDomainPromotionTemplate(model))
	}
	return templates, nil
}

func (r *gormPromotionTemplateRepository) Create(ctx context.Context, template *domain.PromotionTemplate) error {
	model := toGormPromotionTemplate(template)
	return r.db.WithContext(ctx).Create(model).Error
}

func (r *gormPromotionTemplateRepository) Update(ctx context.Context, template *domain.PromotionTemplate) error {
	model := toGormPromotionTemplate(template)
	// GORM's Save will update all fields when a primary key is present,
	// or create a new record if it's missing.
	return r.db.WithContext(ctx).Save(model).Error
}
