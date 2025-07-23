// promotion-service/internal/infrastructure/discount/fixed_amount_strategy.go
package discount

import (
	"encoding/json"
	"fmt"
	"nexus/internal/service/promotion/domain"
)

// FixedAmountStrategyProperties 定义了满减策略所需的参数结构。
// 它将用于从 PromotionTemplate 的 DiscountProperties JSON字段中反序列化数据。
type FixedAmountStrategyProperties struct {
	Threshold int64 `json:"threshold"` // 满减门槛（单位：分）
	Amount    int64 `json:"amount"`    // 优惠金额（单位：分）
}

// FixedAmountStrategy 实现了 domain.DiscountStrategy 接口，用于处理满减/立减优惠。
type FixedAmountStrategy struct{}

func (s *FixedAmountStrategy) Calculate(fact domain.Fact, template *domain.PromotionTemplate) (*domain.DiscountApplication, error) {
	var props FixedAmountStrategyProperties
	if err := json.Unmarshal([]byte(template.DiscountProperties), &props); err != nil {
		return nil, fmt.Errorf("failed to parse fixed amount properties: %w", err)
	}

	// 检查是否达到满减门槛
	if fact.TotalAmount < props.Threshold {
		return &domain.DiscountApplication{Amount: 0}, nil // 未达到门槛，不优惠
	}

	return &domain.DiscountApplication{
		Amount:       props.Amount,
		StrategyName: "FixedAmountStrategy",
		Description:  fmt.Sprintf("满%d.%02d元减%d.%02d元", props.Threshold/100, props.Threshold%100, props.Amount/100, props.Amount%100),
	}, nil
}
