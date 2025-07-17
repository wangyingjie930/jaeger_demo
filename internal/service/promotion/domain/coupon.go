package domain

import (
	"errors"
	"time"
)

// 错误定义
var (
	ErrCouponNotFound      = errors.New("coupon not found")
	ErrCouponNotApplicable = errors.New("coupon is not applicable for this order")
	ErrCouponAlreadyUsed   = errors.New("coupon has already been used")
	ErrCouponExpired       = errors.New("coupon has expired")
	ErrCouponStatusInvalid = errors.New("coupon status is invalid for this operation")
)

// CouponType 优惠券类型
type CouponType int

const (
	TypeFixedAmount CouponType = 1 // 满减
	TypeDiscount    CouponType = 2 // 折扣
	TypeNoThreshold CouponType = 3 // 无门槛
)

// CouponScopeType 适用范围类型
type CouponScopeType int

const (
	ScopeAll         CouponScopeType = 1 // 全场
	ScopeCategory    CouponScopeType = 2 // 指定商品分类
	ScopeSpecificSKU CouponScopeType = 3 // 指定商品
)

// UserCouponStatus 用户优惠券状态
type UserCouponStatus int

const (
	StatusUnused  UserCouponStatus = 1 // 未使用
	StatusUsed    UserCouponStatus = 2 // 已使用
	StatusExpired UserCouponStatus = 3 // 已过期
	StatusFrozen  UserCouponStatus = 4 // 已冻结 (SAGA)
)

// CouponTemplate 是优惠券的模板，定义了优惠券的规则
type CouponTemplate struct {
	ID              int64
	TemplateCode    string
	Name            string
	Type            CouponType
	DiscountValue   float64
	ThresholdAmount float64
	ScopeType       CouponScopeType
	ScopeValue      []string // 例如商品ID列表
}

// UserCoupon 是用户领取的优惠券实例
type UserCoupon struct {
	ID         int64
	CouponCode string
	UserID     string
	Template   *CouponTemplate
	Status     UserCouponStatus
	ValidTo    time.Time
}

// CanUse 检查优惠券是否可用于给定的订单
func (uc *UserCoupon) CanUse(orderAmount float64, itemIDs []string) (float64, error) {
	if uc.Status != StatusUnused {
		return 0, ErrCouponStatusInvalid
	}
	if time.Now().After(uc.ValidTo) {
		return 0, ErrCouponExpired
	}
	if orderAmount < uc.Template.ThresholdAmount {
		return 0, ErrCouponNotApplicable
	}

	// ✨ [核心改造] 检查适用范围
	isApplicable := false
	switch uc.Template.ScopeType {
	case ScopeAll:
		isApplicable = true
	case ScopeSpecificSKU:
		// 如果订单中的任何一个商品ID在优惠券的适用范围内，则认为可用
		itemScope := make(map[string]struct{})
		for _, id := range uc.Template.ScopeValue {
			itemScope[id] = struct{}{}
		}
		for _, orderItemID := range itemIDs {
			if _, ok := itemScope[orderItemID]; ok {
				isApplicable = true
				break
			}
		}
	case ScopeCategory:
		// 这是一个更复杂的场景，需要查询商品属于哪个分类，然后与ScopeValue进行匹配
	default:
		isApplicable = false // 未知范围类型，默认不可用
	}

	if !isApplicable {
		return 0, ErrCouponNotApplicable
	}

	// 计算优惠金额
	var discountAmount float64
	switch uc.Template.Type {
	case TypeFixedAmount, TypeNoThreshold:
		discountAmount = uc.Template.DiscountValue
	case TypeDiscount:
		// 注意：折扣券的DiscountValue通常是折扣率，比如0.8代表八折
		// 计算出的应该是优惠掉的金额
		discountAmount = orderAmount * (1 - uc.Template.DiscountValue)
		if discountAmount < 0 {
			discountAmount = 0
		}
	}

	return discountAmount, nil
}
