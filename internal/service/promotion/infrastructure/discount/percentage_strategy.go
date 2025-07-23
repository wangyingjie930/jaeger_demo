// promotion-service/internal/infrastructure/discount/percentage_strategy.go
package discount

import (
	"encoding/json"
	"fmt"
	"nexus/internal/service/promotion/domain"
)

// PercentageStrategyProperties 定义了折扣策略所需的参数结构。
type PercentageStrategyProperties struct {
	Percentage int32 `json:"percentage"` // 折扣率，例如88代表8.8折
	Ceiling    int64 `json:"ceiling"`    // 封顶金额（单位：分），可选，0代表不封顶
}

// PercentageStrategy 实现了 domain.DiscountStrategy 接口，用于处理折扣优惠。
type PercentageStrategy struct{}

func (s *PercentageStrategy) Calculate(fact domain.Fact, template *domain.PromotionTemplate) (*domain.DiscountApplication, error) {
	var props PercentageStrategyProperties
	if err := json.Unmarshal([]byte(template.DiscountProperties), &props); err != nil {
		return nil, fmt.Errorf("failed to parse percentage properties: %w", err)
	}

	if props.Percentage <= 0 || props.Percentage >= 100 {
		return nil, fmt.Errorf("invalid percentage value: %d", props.Percentage)
	}

	// 计算优惠金额
	discountAmount := fact.TotalAmount * (100 - int64(props.Percentage)) / 100

	// 检查是否超过封顶金额
	if props.Ceiling > 0 && discountAmount > props.Ceiling {
		discountAmount = props.Ceiling
	}

	return &domain.DiscountApplication{
		Amount:       discountAmount,
		StrategyName: "PercentageStrategy",
		Description:  fmt.Sprintf("享受%d.%d折优惠", props.Percentage/10, props.Percentage%10),
	}, nil
}
