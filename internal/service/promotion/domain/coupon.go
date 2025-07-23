// promotion-service/internal/domain/coupon.go
package domain

import "time"

// UserCouponStatus 定义了用户优惠券的生命周期状态。
// 我们引入了Frozen状态来处理SAGA事务的中间态。
type UserCouponStatus string

const (
	StatusUnused  UserCouponStatus = "UNUSED"  // 未使用
	StatusFrozen  UserCouponStatus = "FROZEN"  // 冻结中（下单但未支付）
	StatusUsed    UserCouponStatus = "USED"    // 已使用
	StatusExpired UserCouponStatus = "EXPIRED" // 已过期
)

// UserCoupon 代表一个用户持有的一张具体的优惠券实例。
type UserCoupon struct {
	ID         int64
	UserID     int64
	Status     UserCouponStatus
	ReceivedAt time.Time
	UsedAt     time.Time
	ExpiredAt  time.Time

	// 关键关联：指向一个特定版本的优惠模板。
	// 这确保了即使用户领取后，管理员修改了活动规则，
	// 用户手中的券的权益仍然被锁定在领取时的版本。
	TemplateID      int64
	TemplateVersion int32
}

// IsAvailable 检查优惠券当前是否可用（非终态）。
func (uc *UserCoupon) IsAvailable() bool {
	return uc.Status == StatusUnused && time.Now().Before(uc.ExpiredAt)
}

// Freeze 将优惠券状态置为冻结，用于SAGA流程。
func (uc *UserCoupon) Freeze() {
	// 在此可以添加状态转换的保护逻辑
	if uc.Status == StatusUnused {
		uc.Status = StatusFrozen
	}
}

// Unfreeze 解冻优惠券，用于SAGA回滚。
func (uc *UserCoupon) Unfreeze() {
	if uc.Status == StatusFrozen {
		uc.Status = StatusUnused
	}
}
