package adapter

import (
	"context"
	"net/url"
	"nexus/internal/pkg/constants"
	"nexus/internal/pkg/httpclient"
)

// PricingHTTPAdapter 实现了 port.PricingService 和 port.ShippingService 接口。
// 在这个例子中，我们将它们合并在一个适配器中，因为它们都是简单的HTTP GET/POST。
type PricingHTTPAdapter struct {
	client *httpclient.Client
}

// NewPricingHTTPAdapter 创建一个新的定价/运费适配器。
func NewPricingHTTPAdapter(client *httpclient.Client) *PricingHTTPAdapter {
	return &PricingHTTPAdapter{client: client}
}

// CalculatePrice 实现了调用定价或促销服务的逻辑。
func (a *PricingHTTPAdapter) CalculatePrice(ctx context.Context, userID string, isVIP bool, promoID string) (float64, error) {
	params := url.Values{}
	isVIPStr := "false"
	if isVIP {
		isVIPStr = "true"
	}
	params.Set("is_vip", isVIPStr)
	params.Set("user_id", userID)

	var err error
	if promoID != "" {
		// 如果有促销ID，则调用促销服务
		err = a.client.CallService(ctx, constants.PromotionService, constants.PromotionGetPromoPricePath, params)
	} else {
		// 否则调用标准定价服务
		err = a.client.CallService(ctx, constants.PricingService, constants.PricingCalculatePricePath, params)
	}

	if err != nil {
		return 0, err
	}

	// 注意：在实际应用中，HTTP响应体应该包含计算出的价格，这里为了简化，我们返回一个固定值。
	return 99.99, nil
}

// GetQuote 实现了调用运费服务的逻辑。
func (a *PricingHTTPAdapter) GetQuote(ctx context.Context) (float64, error) {
	err := a.client.CallService(ctx, constants.ShippingService, constants.ShippingGetQuotePath, url.Values{})
	if err != nil {
		return 0, err
	}
	// 同样，这里简化处理，返回一个固定值。
	return 10.0, nil
}
