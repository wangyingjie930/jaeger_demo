package application

import (
	"context"
	"nexus/internal/service/promotion/domain"
)

// internal/service/promotion/application/service.go

type PromotionService interface {
	// CreatePromotionTemplate 创建一个新的促销活动模板
	// 这是所有规则的起点
	CreatePromotionTemplate(ctx context.Context, req *CreateTemplateRequest) (*TemplateResponse, error)

	// UpdatePromotionTemplate 更新一个促销活动模板
	// 内部实现应是创建新版本，而不是修改旧版本
	UpdatePromotionTemplate(ctx context.Context, req *UpdateTemplateRequest) (*TemplateResponse, error)

	// DeactivatePromotionTemplate 停用一个促销活动
	DeactivatePromotionTemplate(ctx context.Context, templateGroupID string) error

	// GetPromotionTemplate 获取一个促销活动的具体版本详情
	GetPromotionTemplate(ctx context.Context, templateID int64) (*TemplateResponse, error)

	// GetActiveTemplateByGroup 获取一个活动当前生效的版本
	GetActiveTemplateByGroup(ctx context.Context, templateGroupID string) (*TemplateResponse, error)

	// IssueCouponToUser 为指定用户发放一张优惠券
	// 这是最核心的发券接口
	IssueCouponToUser(ctx context.Context, req *IssueCouponRequest) (*UserCouponResponse, error)

	// IssueCouponsInBatch 批量为用户发放优惠券
	IssueCouponsInBatch(ctx context.Context, req *BatchIssueCouponRequest) error

	// CalculateBestOffer 评估并计算最优优惠
	// 这是规则引擎的核心价值所在，也是性能要求最高的接口
	// 它接收一个“事实”对象，包含了计算所需的所有上下文 [cite: 39]
	CalculateBestOffer(ctx context.Context, fact *domain.Fact) (*DiscountApplication, error)

	// GetApplicableCoupons 获取用户在当前“事实”下所有可用的优惠券列表
	// 用于在购物车或结算页向用户展示可用优惠券
	GetApplicableCoupons(ctx context.Context, fact *domain.Fact, userID int64) ([]*UserCouponResponse, error)

	// FreezeUserCoupon 冻结用户优惠券（SAGA事务-预备）
	// 在订单创建但未支付时调用
	FreezeUserCoupon(ctx context.Context, userID int64, couponCode string) error

	// UseUserCoupon 核销（使用）用户优惠券（SAGA事务-确认）
	// 在订单支付成功后调用
	UseUserCoupon(ctx context.Context, userID int64, couponCode string) error

	// UnfreezeUserCoupon 解冻用户优惠券（SAGA事务-回滚）
	// 在订单取消或支付超时后调用
	UnfreezeUserCoupon(ctx context.Context, userID int64, couponCode string) error
}
