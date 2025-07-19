package saga

import (
	"context"
	"fmt"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// InventoryHandler 负责库存预占步骤。
type InventoryHandler struct {
	NextHandler
}

func (h *InventoryHandler) Handle(orderCtx *OrderContext) error {
	ctx, span := orderCtx.Tracer.Start(orderCtx.Ctx, "saga.InventoryReserve")
	defer span.End()

	fmt.Println("【Saga】=> 步骤 2: 预占库存...")

	itemsToReserve := map[string]int{}
	for _, item := range orderCtx.Order.Items {
		itemsToReserve[item] = orderCtx.Order.Quantity
	}

	span.SetAttributes(attribute.StringSlice("items", orderCtx.Order.Items))

	// 【核心改造】: 调用抽象接口。
	if rollback, err := orderCtx.InventoryService.ReserveStock(ctx, orderCtx.Order.ID, itemsToReserve); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Inventory reservation failed")

		// 【核心改造】: 注册的补偿操作也依赖于抽象接口。
		orderCtx.AddCompensation(func(compCtx context.Context) {
			compCtx, compSpan := orderCtx.Tracer.Start(compCtx, "saga.compensation.ReleaseStock")
			defer compSpan.End()

			compSpan.SetAttributes(attribute.StringSlice("items", orderCtx.Order.Items))

			// 补偿失败需要记录严重错误，并可能需要人工介入
			if err := orderCtx.InventoryService.ReleaseStock(compCtx, orderCtx.Order.ID, rollback); err != nil {
				compSpan.RecordError(err)
			}
		})

		return err
	}

	span.AddEvent("All Items reserved successfully")

	return h.executeNext(orderCtx)
}
