package application

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"nexus/internal/service/promotion/domain"
	"nexus/internal/service/promotion/infrastructure/discount"
	"nexus/internal/service/promotion/infrastructure/rule"
)

// promotionServiceImpl 是 PromotionService 的实现
type promotionServiceImpl struct {
	templateRepo domain.PromotionTemplateRepository
	couponRepo   domain.CouponRepository
	ruleEngine   domain.RuleEngine
	strategyFty  *discount.StrategyFactory
}

// NewPromotionService 创建一个新的 PromotionService 实例
func NewPromotionService(
	templateRepo domain.PromotionTemplateRepository,
	couponRepo domain.CouponRepository,
) PromotionService {
	return &promotionServiceImpl{
		templateRepo: templateRepo,
		couponRepo:   couponRepo,
		ruleEngine:   rule.NewJSONRuleEngineAdapter(), // 直接实例化基础设施层的具体实现
		strategyFty:  discount.NewStrategyFactory(),   // 工厂模式 [cite: 156]
	}
}

// CreatePromotionTemplate 实现了不可变性设计 [cite: 215]
func (s *promotionServiceImpl) CreatePromotionTemplate(ctx context.Context, req *CreateTemplateRequest) (*TemplateResponse, error) {
	template := &domain.PromotionTemplate{
		TemplateGroupID:    uuid.New().String(), // 创建时生成新的组ID
		Version:            1,                   // 初始版本为1
		Name:               req.Name,
		Description:        req.Description,
		PromotionType:      req.PromotionType,
		RuleDefinition:     req.RuleDefinition,
		DiscountType:       domain.DiscountType(req.DiscountType),
		DiscountProperties: req.DiscountProperties,
		StartDate:          req.StartDate,
		EndDate:            req.EndDate,
		IsExclusive:        req.IsExclusive,
		Priority:           req.Priority,
		IsActive:           true, // 默认激活
	}

	if err := s.templateRepo.Create(ctx, template); err != nil {
		return nil, err
	}

	return toTemplateResponse(template), nil
}

// UpdatePromotionTemplate 通过创建新版本来实现更新，遵循不可变性原则 [cite: 217, 218]
func (s *promotionServiceImpl) UpdatePromotionTemplate(ctx context.Context, req *UpdateTemplateRequest) (*TemplateResponse, error) {
	// 1. 找到最新的版本
	latest, err := s.templateRepo.FindLatestByGroupID(ctx, req.TemplateGroupID)
	if err != nil {
		return nil, err
	}
	if latest == nil {
		return nil, fmt.Errorf("promotion template group with ID %s not found", req.TemplateGroupID)
	}

	// 2. 停用旧版本 (如果当前是激活的)
	if latest.IsActive {
		latest.IsActive = false
		if err := s.templateRepo.Update(ctx, latest); err != nil {
			return nil, err
		}
	}

	// 3. 创建新版本
	newVersion := &domain.PromotionTemplate{
		TemplateGroupID:    latest.TemplateGroupID,
		Version:            latest.Version + 1, // 版本号递增
		Name:               req.Name,
		Description:        req.Description,
		PromotionType:      latest.PromotionType, // 类型等核心属性不可变
		RuleDefinition:     req.RuleDefinition,
		DiscountType:       domain.DiscountType(req.DiscountType),
		DiscountProperties: req.DiscountProperties,
		StartDate:          req.StartDate,
		EndDate:            req.EndDate,
		IsExclusive:        req.IsExclusive,
		Priority:           req.Priority,
		IsActive:           true, // 新版本默认为激活状态
	}

	if err := s.templateRepo.Create(ctx, newVersion); err != nil {
		// 理想情况下这里应该有事务回滚
		return nil, err
	}

	return toTemplateResponse(newVersion), nil
}

// DeactivatePromotionTemplate 停用整个模板组
func (s *promotionServiceImpl) DeactivatePromotionTemplate(ctx context.Context, templateGroupID string) error {
	activeTpl, err := s.templateRepo.FindActiveByGroupID(ctx, templateGroupID)
	if err != nil {
		return err
	}
	if activeTpl == nil {
		return nil // 已经没有激活的了
	}

	activeTpl.IsActive = false
	return s.templateRepo.Update(ctx, activeTpl)
}

func (s *promotionServiceImpl) GetPromotionTemplate(ctx context.Context, templateID int64) (*TemplateResponse, error) {
	template, err := s.templateRepo.FindByID(ctx, templateID)
	if err != nil {
		return nil, err
	}
	return toTemplateResponse(template), nil
}

func (s *promotionServiceImpl) GetActiveTemplateByGroup(ctx context.Context, templateGroupID string) (*TemplateResponse, error) {
	template, err := s.templateRepo.FindActiveByGroupID(ctx, templateGroupID)
	if err != nil {
		return nil, err
	}
	return toTemplateResponse(template), nil
}

func (s *promotionServiceImpl) IssueCouponToUser(ctx context.Context, req *IssueCouponRequest) (*UserCouponResponse, error) {
	// 1. 确认模板存在且有效
	template, err := s.templateRepo.FindByID(ctx, req.TemplateID)
	if err != nil {
		return nil, err
	}
	if template == nil || !template.IsActive || time.Now().After(template.EndDate) {
		return nil, fmt.Errorf("template %d is not valid for issuance", req.TemplateID)
	}

	// 2. 创建优惠券实例
	coupon := &domain.UserCoupon{
		UserID:     req.UserID,
		CouponCode: uuid.New().String(), // 生成唯一的券码
		TemplateID: template.ID,
		Status:     domain.StatusUnused,
		IssueDate:  time.Now(),
		ExpiryDate: template.EndDate, // 可根据业务调整，例如“领取后30天有效”
	}

	if err := s.couponRepo.Save(ctx, coupon); err != nil {
		return nil, err
	}

	return toUserCouponResponse(coupon), nil
}

func (s *promotionServiceImpl) IssueCouponsInBatch(ctx context.Context, req *BatchIssueCouponRequest) error {
	// 此处为简化实现，实际大厂中会使用任务队列异步处理
	g, gCtx := errgroup.WithContext(ctx)
	for _, userID := range req.UserIDs {
		uid := userID // a copy of userID for the goroutine
		g.Go(func() error {
			_, err := s.IssueCouponToUser(gCtx, &IssueCouponRequest{
				TemplateID: req.TemplateID,
				UserID:     uid,
			})
			return err
		})
	}
	return g.Wait()
}

// CalculateBestOffer 实现了择优逻辑 [cite: 92]
func (s *promotionServiceImpl) CalculateBestOffer(ctx context.Context, fact *domain.Fact) (*DiscountApplicationResponse, error) {
	coupons, err := s.GetApplicableCoupons(ctx, fact, fact.User.ID)
	if err != nil {
		return nil, err
	}

	if len(coupons) == 0 {
		// 返回DTO响应
		return toDiscountApplicationResponse(&domain.DiscountApplication{Amount: 0, Description: "无可用优惠"}), nil
	}

	var bestOffer *domain.DiscountApplication
	var bestOfferAmount int64 = 0

	for _, couponResp := range coupons {
		template, err := s.templateRepo.FindByID(ctx, couponResp.TemplateID)
		if err != nil || template == nil {
			continue // 跳过无效模板
		}

		// 使用策略模式计算优惠
		strategy, err := s.strategyFty.CreateStrategy(template.DiscountType)
		if err != nil {
			continue
		}

		offer, err := strategy.Calculate(*fact, template)
		if err != nil {
			continue
		}

		// 择优：选择优惠金额最大的
		if offer.Amount > bestOfferAmount {
			bestOffer = offer
			bestOfferAmount = offer.Amount
		}
	}

	if bestOffer == nil {
		// 返回DTO响应
		return toDiscountApplicationResponse(&domain.DiscountApplication{Amount: 0, Description: "无可用优惠"}), nil
	}
	// 返回DTO响应
	return toDiscountApplicationResponse(bestOffer), nil
}

// GetApplicableCoupons 筛选出在当前Fact下所有可用的优惠券
func (s *promotionServiceImpl) GetApplicableCoupons(ctx context.Context, fact *domain.Fact, userID int64) ([]*UserCouponResponse, error) {
	// 1. 获取用户所有未使用的优惠券
	userCoupons, err := s.couponRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	availableCoupons := make([]*domain.UserCoupon, 0)
	for _, c := range userCoupons {
		if c.IsAvailable() {
			availableCoupons = append(availableCoupons, c)
		}
	}

	if len(availableCoupons) == 0 {
		return nil, nil
	}

	// 2. 并发检查每张券的规则是否满足 [cite: 159]
	applicableCoupons := struct {
		sync.Mutex
		data []*UserCouponResponse
	}{}

	g, gCtx := errgroup.WithContext(ctx)
	for _, coupon := range availableCoupons {
		c := coupon // copy
		g.Go(func() error {
			template, err := s.templateRepo.FindByID(gCtx, c.TemplateID)
			if err != nil || template == nil {
				return nil // 跳过无效模板
			}

			// 使用规则引擎评估LHS
			satisfied, err := s.ruleEngine.Evaluate(template.RuleDefinition, *fact)
			if err != nil || !satisfied {
				return nil // 规则不满足
			}

			// 如果满足，则加入到最终列表
			applicableCoupons.Lock()
			applicableCoupons.data = append(applicableCoupons.data, toUserCouponResponse(c))
			applicableCoupons.Unlock()

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// 按优先级和优惠金额排序 (可选，但体验更好)
	sort.Slice(applicableCoupons.data, func(i, j int) bool {
		// ... 这里可以加入更复杂的排序逻辑
		return applicableCoupons.data[i].ID > applicableCoupons.data[j].ID
	})

	return applicableCoupons.data, nil
}

// --- SAGA 事务方法 ---

func (s *promotionServiceImpl) FreezeUserCoupon(ctx context.Context, userID int64, couponCode string) error {
	coupon, err := s.couponRepo.FindByCode(ctx, couponCode)
	if err != nil {
		return err
	}
	if coupon == nil || coupon.UserID != userID {
		return fmt.Errorf("coupon %s not found or does not belong to user %d", couponCode, userID)
	}
	if !coupon.IsAvailable() {
		return fmt.Errorf("coupon %s is not available", couponCode)
	}

	coupon.Freeze() // 领域方法
	return s.couponRepo.Update(ctx, coupon)
}

func (s *promotionServiceImpl) UseUserCoupon(ctx context.Context, userID int64, couponCode string) error {
	coupon, err := s.couponRepo.FindByCode(ctx, couponCode)
	if err != nil {
		return err
	}
	if coupon == nil || coupon.UserID != userID {
		return fmt.Errorf("coupon %s not found or does not belong to user %d", couponCode, userID)
	}
	// 确认是冻结状态才能使用，保证流程正确性
	if coupon.Status != domain.StatusFrozen {
		return fmt.Errorf("coupon %s is not in FROZEN state", couponCode)
	}

	coupon.Status = domain.StatusUsed
	now := time.Now()
	coupon.UsedAt = &now
	return s.couponRepo.Update(ctx, coupon)
}

func (s *promotionServiceImpl) UnfreezeUserCoupon(ctx context.Context, userID int64, couponCode string) error {
	coupon, err := s.couponRepo.FindByCode(ctx, couponCode)
	if err != nil {
		return err
	}
	if coupon == nil || coupon.UserID != userID {
		return fmt.Errorf("coupon %s not found or does not belong to user %d", couponCode, userID)
	}
	if coupon.Status != domain.StatusFrozen {
		// 如果不是冻结状态，可能意味着流程已结束或异常，直接返回成功，保证幂等性
		return nil
	}
	coupon.Unfreeze() // 领域方法
	return s.couponRepo.Update(ctx, coupon)
}
