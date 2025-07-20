package application

import (
	"context"
	"fmt"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/trace"
	"nexus/internal/pkg/logger"
	"nexus/internal/service/promotion/domain"
	"time"
)

// PromotionService 定义了优惠服务提供的所有业务用例
type PromotionService struct {
	couponRepo domain.CouponRepository
	tracer     trace.Tracer
}

// NewPromotionService 创建一个新的优惠服务实例
func NewPromotionService(repo domain.CouponRepository, tracer trace.Tracer) *PromotionService {
	return &PromotionService{
		couponRepo: repo,
		tracer:     tracer,
	}
}

// GetPromoPrice 计算促销活动价格
func (s *PromotionService) GetPromoPrice(ctx context.Context, isVip bool, userID string) (*GetPromoPriceResponse, error) {
	ctx, span := s.tracer.Start(ctx, "service.GetPromoPrice")
	defer span.End()

	// 从 Baggage 中提取业务上下文
	promoID := baggage.FromContext(ctx).Member("promotion_id").Value()

	span.SetAttributes(
		attribute.Bool("user.is_vip", isVip),
		attribute.String("promotion.id", promoID),
		attribute.String("user.id", userID),
	)

	logger.Ctx(ctx).Printf("Calculating promo price for promotion '%s'", promoID)

	if promoID == "" {
		return nil, fmt.Errorf("promotion_id is missing from baggage")
	}

	// 模拟促销价格计算的耗时
	time.Sleep(120 * time.Millisecond)

	span.AddEvent("Promotion price calculated successfully")

	return &GetPromoPriceResponse{
		Price: 79.99,
		Promo: promoID,
	}, nil
}

// UseCoupon 是核销优惠券的核心业务逻辑
func (s *PromotionService) UseCoupon(ctx context.Context, req *UseCouponRequest) (*UseCouponResponse, error) {
	ctx, span := s.tracer.Start(ctx, "service.UseCoupon")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", req.UserID),
		attribute.String("coupon.code", req.CouponCode),
		attribute.String("order.id", req.OrderID),
	)

	// 1. 从仓储获取优惠券实体
	userCoupon, err := s.couponRepo.FindByCode(ctx, req.CouponCode)
	if err != nil {
		span.RecordError(err)
		return nil, err // 优惠券不存在
	}

	// 2. 调用领域对象的业务方法进行校验
	discount, err := userCoupon.CanUse(req.OrderAmount, req.ItemIDs)
	if err != nil {
		span.RecordError(err)
		return nil, err // 优惠券不可用
	}

	// 3. 【核心】将优惠券状态更新为"冻结"
	userCoupon.Status = domain.StatusFrozen
	// 在SAGA模式中，这里只更新状态为FROZEN，并不会最终置为USED
	// 支付成功后，会有一个消息来驱动将状态从FROZEN更新为USED
	// 如果订单最终失败，补偿逻辑会将状态从FROZEN回滚为UNUSUED

	// 4. 持久化状态变更
	if err := s.couponRepo.Save(ctx, userCoupon); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to save coupon status: %w", err)
	}

	logger.Ctx(ctx).Printf("Coupon %s for order %s has been used (frozen).", req.CouponCode, req.OrderID)

	resp := &UseCouponResponse{
		Success:        true,
		DiscountAmount: discount,
		FinalAmount:    req.OrderAmount - discount,
		Message:        "Coupon applied successfully",
	}

	return resp, nil
}

// ✨ [新增] CancelCouponUsage 是 UseCoupon 的补偿方法
// 用于在 SAGA 模式下，当订单处理失败时，回滚优惠券的状态
func (s *PromotionService) CancelCouponUsage(ctx context.Context, req *UseCouponRequest) error {
	ctx, span := s.tracer.Start(ctx, "service.CancelCouponUsage (Compensation)")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", req.UserID),
		attribute.String("coupon.code", req.CouponCode),
		attribute.String("order.id", req.OrderID),
	)

	// 1. 再次获取优惠券
	userCoupon, err := s.couponRepo.FindByCode(ctx, req.CouponCode)
	if err != nil {
		// 如果补偿时券都找不到了，这是一个严重问题，需要记录错误但不能中断其他补偿
		span.RecordError(err)
		return fmt.Errorf("compensation failed: %w", err)
	}

	// 2. 只有处于冻结状态的券才能被回滚
	if userCoupon.Status != domain.StatusFrozen {
		err := fmt.Errorf("coupon status is not 'FROZEN', cannot compensate. Current status: %d", userCoupon.Status)
		span.RecordError(err)
		return err
	}

	// 3. 将状态回滚为“未使用”
	userCoupon.Status = domain.StatusUnused
	if err := s.couponRepo.Save(ctx, userCoupon); err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to save coupon status during compensation: %w", err)
	}

	logger.Ctx(ctx).Printf("Compensation: Coupon %s for order %s has been rolled back to UNUSED.", req.CouponCode, req.OrderID)
	span.AddEvent("Coupon status rolled back to UNUSED")

	return nil
}
