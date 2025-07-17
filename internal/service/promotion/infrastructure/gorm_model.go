package infrastructure

import (
	"database/sql"
	"gorm.io/gorm"
	"nexus/internal/service/promotion/domain"
	"time"
)

// CouponTemplateModel 对应数据库中的 coupon_template 表
type CouponTemplateModel struct {
	gorm.Model
	TemplateCode    string
	Name            string
	Description     string
	Type            domain.CouponType `gorm:"type:tinyint"`
	DiscountValue   float64           `gorm:"type:decimal(10,2)"`
	ThresholdAmount float64           `gorm:"type:decimal(10,2)"`
	ScopeType       domain.CouponScopeType
	ScopeValue      string `gorm:"type:text"`
	TotalQuantity   int64
	IssuedQuantity  int64
	ValidityType    int
	ValidFrom       time.Time
	ValidTo         time.Time
	ValidDays       int
	Status          int
}

// TableName 指定 GORM 应该使用的表名
func (CouponTemplateModel) TableName() string {
	return "coupon_template"
}

// UserCouponModel 对应数据库中的 user_coupon 表
type UserCouponModel struct {
	gorm.Model
	CouponCode string `gorm:"uniqueIndex"`
	UserID     string
	TemplateID uint
	Status     domain.UserCouponStatus `gorm:"type:tinyint;default:1"`
	OrderID    sql.NullString
	ValidFrom  time.Time
	ValidTo    time.Time
	UsedAt     sql.NullTime
	// 关联关系
	Template CouponTemplateModel `gorm:"foreignKey:TemplateID"`
}

// TableName 指定 GORM 应该使用的表名
func (UserCouponModel) TableName() string {
	return "user_coupon"
}
