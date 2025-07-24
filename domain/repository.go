package domain

import "context"

// CouponRepository 定义了优惠券数据的持久化接口
// 这是领域层与基础设施层之间的“插座”
type CouponRepository interface {
	FindByCode(ctx context.Context, code string) (*UserCoupon, error)
	Save(ctx context.Context, coupon *UserCoupon) error
}
