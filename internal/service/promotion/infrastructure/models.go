// promotion-service/internal/infrastructure/persistence/models.go
package persistence

import "time"

// PromotionTemplateModel 是 PromotionTemplate 领域对象在数据库中的表示。
type PromotionTemplateModel struct {
	ID                 int64 `gorm:"primaryKey"`
	Version            int32
	Name               string
	Description        string
	Status             string
	RuleDefinition     string `gorm:"type:json"`
	DiscountType       string
	DiscountProperties string `gorm:"type:json"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (PromotionTemplateModel) TableName() string {
	return "promotion_templates"
}

// UserCouponModel 是 UserCoupon 领域对象在数据库中的表示。
type UserCouponModel struct {
	ID              int64 `gorm:"primaryKey"`
	UserID          int64
	Status          string
	ReceivedAt      time.Time
	UsedAt          time.Time `gorm:"default:null"`
	ExpiredAt       time.Time
	TemplateID      int64
	TemplateVersion int32
}

func (UserCouponModel) TableName() string {
	return "user_coupons"
}
