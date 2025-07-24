package application

import (
	"nexus/internal/service/promotion/domain"
	"time"
)

// CreateTemplateRequest 定义了创建新促销模板时所需的输入。
type CreateTemplateRequest struct {
	Name               string    `json:"name"`
	Description        string    `json:"description"`
	PromotionType      string    `json:"promotion_type"`
	RuleDefinition     string    `json:"rule_definition"`
	DiscountType       string    `json:"discount_type"`
	DiscountProperties string    `json:"discount_properties"`
	StartDate          time.Time `json:"start_date"`
	EndDate            time.Time `json:"end_date"`
	IsExclusive        bool      `json:"is_exclusive"`
	Priority           int       `json:"priority"`
}

// UpdateTemplateRequest 定义了更新促销模板时所需的输入。
// 注意，这里使用 TemplateGroupID 来标识一个活动的集合，而不是单个版本。
type UpdateTemplateRequest struct {
	TemplateGroupID    string    `json:"template_group_id"`
	Name               string    `json:"name"`
	Description        string    `json:"description"`
	RuleDefinition     string    `json:"rule_definition"`
	DiscountType       string    `json:"discount_type"`
	DiscountProperties string    `json:"discount_properties"`
	StartDate          time.Time `json:"start_date"`
	EndDate            time.Time `json:"end_date"`
	IsExclusive        bool      `json:"is_exclusive"`
	Priority           int       `json:"priority"`
}

// TemplateResponse 是返回给客户端的促销模板视图。
// 它屏蔽了内部领域模型的复杂性。
type TemplateResponse struct {
	ID                 int64     `json:"id"`
	TemplateGroupID    string    `json:"template_group_id"`
	Version            int32     `json:"version"`
	Name               string    `json:"name"`
	Description        string    `json:"description"`
	PromotionType      string    `json:"promotion_type"`
	RuleDefinition     string    `json:"rule_definition"`
	DiscountType       string    `json:"discount_type"`
	DiscountProperties string    `json:"discount_properties"`
	StartDate          time.Time `json:"start_date"`
	EndDate            time.Time `json:"end_date"`
	IsExclusive        bool      `json:"is_exclusive"`
	Priority           int       `json:"priority"`
	IsActive           bool      `json:"is_active"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// IssueCouponRequest 定义了向单个用户发券的请求。
type IssueCouponRequest struct {
	TemplateID int64 `json:"template_id"`
	UserID     int64 `json:"user_id"`
}

// BatchIssueCouponRequest 定义了批量发券的请求。
type BatchIssueCouponRequest struct {
	TemplateID int64   `json:"template_id"`
	UserIDs    []int64 `json:"user_ids"`
}

// UserCouponResponse 是返回给客户端的用户优惠券视图。
type UserCouponResponse struct {
	ID         int64                   `json:"id"`
	UserID     int64                   `json:"user_id"`
	CouponCode string                  `json:"coupon_code"`
	TemplateID int64                   `json:"template_id"`
	Status     domain.UserCouponStatus `json:"status"`
	IssueDate  time.Time               `json:"issue_date"`
	ExpiryDate time.Time               `json:"expiry_date"`
	UsedAt     *time.Time              `json:"used_at,omitempty"`
}

// DiscountApplicationResponse 是优惠计算结果的DTO。
// 用于向调用方展示计算出的优惠详情。
type DiscountApplicationResponse struct {
	Amount       int64  `json:"amount"`        // 优惠的总金额
	StrategyName string `json:"strategy_name"` // 应用的策略名称
	Description  string `json:"description"`   // 优惠的描述
}

// --- Mapper Functions ---

// toTemplateResponse 将领域对象转换为DTO
func toTemplateResponse(d *domain.PromotionTemplate) *TemplateResponse {
	if d == nil {
		return nil
	}
	return &TemplateResponse{
		ID:                 d.ID,
		TemplateGroupID:    d.TemplateGroupID,
		Version:            d.Version,
		Name:               d.Name,
		Description:        d.Description,
		PromotionType:      d.PromotionType,
		RuleDefinition:     d.RuleDefinition,
		DiscountType:       string(d.DiscountType),
		DiscountProperties: d.DiscountProperties,
		StartDate:          d.StartDate,
		EndDate:            d.EndDate,
		IsExclusive:        d.IsExclusive,
		Priority:           d.Priority,
		IsActive:           d.IsActive,
		CreatedAt:          d.CreatedAt,
		UpdatedAt:          d.UpdatedAt,
	}
}

// toUserCouponResponse 将领域对象转换为DTO
func toUserCouponResponse(d *domain.UserCoupon) *UserCouponResponse {
	if d == nil {
		return nil
	}
	return &UserCouponResponse{
		ID:         d.ID,
		UserID:     d.UserID,
		CouponCode: d.CouponCode,
		TemplateID: d.TemplateID,
		Status:     d.Status,
		IssueDate:  d.IssueDate,
		ExpiryDate: d.ExpiryDate,
		UsedAt:     d.UsedAt,
	}
}

// toDiscountApplicationResponse 将领域对象转换为DTO
func toDiscountApplicationResponse(d *domain.DiscountApplication) *DiscountApplicationResponse {
	if d == nil {
		return nil
	}
	return &DiscountApplicationResponse{
		Amount:       d.Amount,
		StrategyName: d.StrategyName,
		Description:  d.Description,
	}
}
