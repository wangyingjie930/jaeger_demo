package infrastructure

import (
	"context"
	"gorm.io/gorm"
	"nexus/internal/service/promotion/domain"
)

type gormCouponRepository struct {
	db *gorm.DB
}

func NewGormCouponRepository(db *gorm.DB) domain.CouponRepository {
	return &gormCouponRepository{db: db}
}

func (r *gormCouponRepository) FindByCode(ctx context.Context, code string) (*domain.UserCoupon, error) {
	var model UserCouponModel
	if err := r.db.WithContext(ctx).Where("coupon_code = ?", code).First(&model).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return toDomainUserCoupon(&model), nil
}

func (r *gormCouponRepository) FindByID(ctx context.Context, id int64) (*domain.UserCoupon, error) {
	var model UserCouponModel
	if err := r.db.WithContext(ctx).First(&model, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return toDomainUserCoupon(&model), nil
}

func (r *gormCouponRepository) FindByUserID(ctx context.Context, userID int64) ([]*domain.UserCoupon, error) {
	var models []*UserCouponModel
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&models).Error; err != nil {
		return nil, err
	}

	var coupons []*domain.UserCoupon
	for _, model := range models {
		coupons = append(coupons, toDomainUserCoupon(model))
	}
	return coupons, nil
}

func (r *gormCouponRepository) Save(ctx context.Context, coupon *domain.UserCoupon) error {
	model := toGormUserCoupon(coupon)
	return r.db.WithContext(ctx).Create(model).Error
}

func (r *gormCouponRepository) Update(ctx context.Context, coupon *domain.UserCoupon) error {
	model := toGormUserCoupon(coupon)
	return r.db.WithContext(ctx).Save(model).Error
}
