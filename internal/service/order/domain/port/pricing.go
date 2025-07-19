package port

import "context"

// PricingService 是定价服务的出站端口。
type PricingService interface {
	// CalculatePrice 计算订单的最终价格。
	CalculatePrice(ctx context.Context, userID string, isVIP bool, promoID string) (float64, error)
}

// ShippingService 是运费服务的出站端口。
type ShippingService interface {
	// GetQuote 获取运费报价。
	GetQuote(ctx context.Context) (float64, error)
}
