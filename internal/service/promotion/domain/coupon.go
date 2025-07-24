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
// 这是领域模型，现在包含了所有必要的业务字段。
type UserCoupon struct {
	ID         int64
	UserID     int64
	CouponCode string // [新增] 唯一的券码，用于核销
	Status     UserCouponStatus
	IssueDate  time.Time  // [修正] 命名与GORM模型统一 (原 ReceivedAt)
	ExpiryDate time.Time  // [修正] 命名与GORM模型统一 (原 ExpiredAt)
	UsedAt     *time.Time // [修正] 使用指针类型，允许为NULL (原 time.Time)

	// 关键关联：指向一个特定版本的优惠模板ID。
	// 这确保了即使用户领取后，管理员修改了活动规则，
	// 用户手中的券的权益仍然被锁定在领取时的版本。
	TemplateID int64

	// [移除] TemplateVersion 字段是冗余的，因为 TemplateID 本身就指向一个唯一的、带版本的模板记录。

	// [新增] 添加标准的时间戳字段
	CreatedAt time.Time
	UpdatedAt time.Time
}

// IsAvailable 检查优惠券当前是否可用（非终态）。
func (uc *UserCoupon) IsAvailable() bool {
	// [修正] 使用修正后的字段名 ExpiryDate
	return uc.Status == StatusUnused && time.Now().Before(uc.ExpiryDate)
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
