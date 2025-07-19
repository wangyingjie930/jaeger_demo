package saga

import (
	"errors"
	"fmt"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/codes"
	"sync"
)

// PricingHandler 负责动态定价和运费计算的步骤。
// 【承诺】：此实现严格保留了原始的并发调用逻辑。
type PricingHandler struct {
	NextHandler
}

func (h *PricingHandler) Handle(orderCtx *OrderContext) error {
	ctx, span := orderCtx.Tracer.Start(orderCtx.Ctx, "saga.Pricing")
	defer span.End()

	fmt.Println("【Saga】=> 步骤 3: 动态定价与运费计算...")

	// 从 OpenTelemetry Baggage 中提取促销ID
	b := baggage.FromContext(ctx)
	promoId := b.Member("promotion_id").Value()

	// 【核心改造】: 维持原有的并发调用和错误处理机制
	var wg sync.WaitGroup
	errs := make(chan error, 2) // Channel 用于从 goroutine 中安全地传递错误
	wg.Add(2)

	// Goroutine 1: 计算商品价格
	go func() {
		defer wg.Done()
		// 调用 pricing 端口，它内部已经封装了调用促销或标准定价的逻辑
		_, err := orderCtx.PricingService.CalculatePrice(ctx, orderCtx.Order.UserID, orderCtx.Order.IsVIP, promoId)
		if err != nil {
			errs <- fmt.Errorf("pricing service error: %w", err)
		}
	}()

	// Goroutine 2: 计算运费
	go func() {
		defer wg.Done()
		// 调用 shipping 端口
		_, err := orderCtx.ShippingService.GetQuote(ctx)
		if err != nil {
			errs <- fmt.Errorf("shipping service error: %w", err)
		}
	}()

	// 等待所有并发调用完成
	wg.Wait()
	close(errs) // 关闭 channel，以便下面的 for-range 循环可以结束

	// 聚合所有从 channel 中收到的错误
	var combinedErr error
	for err := range errs {
		combinedErr = errors.Join(combinedErr, err) // Go 1.20+ 的标准方式
	}

	if combinedErr != nil {
		span.RecordError(combinedErr)
		span.SetStatus(codes.Error, "Downstream service call failed during pricing/shipping")
		return combinedErr // 中断 Saga
	}

	span.AddEvent("Pricing and shipping quotes calculated successfully in parallel.")

	return h.executeNext(orderCtx)
}
