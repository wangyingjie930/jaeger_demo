package order

import (
	"errors"
	"fmt"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/codes"
	"net/url"
	"sync"
)

type PriceHandler struct {
	NextHandler
}

var (
	pricingServiceURL   = getEnv("PRICING_SERVICE_URL", "http://localhost:8084/calculate_price")
	promotionServiceURL = getEnv("PROMOTION_SERVICE_URL", "http://localhost:8087/get_promo_price")
	shippingServiceURL  = getEnv("SHIPPING_SERVICE_URL", "http://localhost:8086/get_quote")
)

func (h *PriceHandler) Handle(orderCtx *OrderContext) error {
	ctx, span := orderCtx.HTTPClient.Tracer.Start(orderCtx.Ctx, "handler.PriceHandler")
	defer span.End()

	fmt.Println("【责任链】=> 步骤3: 动态定价...")
	isVIP := ""
	if orderCtx.Event.IsVIP {
		isVIP = "true"
	}
	b := baggage.FromContext(ctx)
	promoId := b.Member("promotion_id").Value()

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)

	// <<<<<<< 改造点: 动态定价 >>>>>>>>>
	go func() {
		defer wg.Done()
		q := url.Values{}
		q.Set("is_vip", isVIP)
		q.Set("user_id", orderCtx.Event.UserID)
		// 根据Baggage决定调用哪个服务
		if promoId != "" {
			span.AddEvent("VIP promotion detected, calling promotion-service")
			if err := orderCtx.HTTPClient.Post(ctx, promotionServiceURL, q); err != nil {
				errs <- fmt.Errorf("promotion service error: %w", err)
			}
		} else {
			span.AddEvent("No promotion, calling standard pricing-service")
			if err := orderCtx.HTTPClient.Post(ctx, pricingServiceURL, q); err != nil {
				errs <- fmt.Errorf("pricing service error: %w", err)
			}
		}
	}()
	// <<<<<<< 改造点结束 >>>>>>>>>

	go func() {
		defer wg.Done()
		if err := orderCtx.HTTPClient.Post(ctx, shippingServiceURL, url.Values{}); err != nil {
			errs <- fmt.Errorf("shipping service error: %w", err)
		}
	}()

	wg.Wait()
	close(errs)

	var combinedErr error
	for err := range errs {
		combinedErr = errors.Join(combinedErr, err)
	}

	if combinedErr != nil {
		span.RecordError(combinedErr)
		span.SetStatus(codes.Error, "Downstream service call failed")

		// ✨ [重大改变] 不再需要知道如何回滚库存，只需将错误向上抛出
		//http.Error(orderCtx.Writer, combinedErr.Error(), http.StatusInternalServerError)
		return combinedErr
	}

	return h.executeNext(orderCtx)
}
