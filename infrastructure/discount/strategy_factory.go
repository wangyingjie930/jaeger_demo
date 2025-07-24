// promotion-service/internal/infrastructure/discount/strategy_factory.go
package discount

import (
	"fmt"
	"nexus/internal/service/promotion/domain"
)

// StrategyFactory 负责创建和提供具体的优惠计算策略实例。
// 这是工厂模式的直接应用，它将策略的创建逻辑与使用逻辑解耦。
type StrategyFactory struct{}

func NewStrategyFactory() *StrategyFactory {
	return &StrategyFactory{}
}

// CreateStrategy 根据传入的优惠类型，返回一个具体的策略实现。
func (f *StrategyFactory) CreateStrategy(discountType domain.DiscountType) (domain.DiscountStrategy, error) {
	switch discountType {
	case domain.DiscountTypeFixedAmount:
		return &FixedAmountStrategy{}, nil
	case domain.DiscountTypePercentage:
		return &PercentageStrategy{}, nil
	// 当需要添加新的优惠类型时，只需在这里增加一个新的case分支。
	// case domain.DiscountTypeBuyOneGetOne:
	// 	return &BuyOneGetOneStrategy{}, nil
	default:
		return nil, fmt.Errorf("unsupported discount type: %s", discountType)
	}
}
