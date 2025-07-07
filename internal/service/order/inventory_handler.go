package order

import (
	"context"
	"fmt"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"net/url"
	"strconv"
)

type InventoryReserveHandler struct {
	NextHandler
}

var (
	inventoryReserveURL = getEnv("INVENTORY_RESERVE_URL", "http://localhost:8082/reserve_stock")
	inventoryReleaseURL = getEnv("INVENTORY_RELEASE_URL", "http://localhost:8082/release_stock")
)

func (h *InventoryReserveHandler) Handle(orderCtx *OrderContext) error {
	ctx, span := orderCtx.HTTPClient.Tracer.Start(orderCtx.Ctx, "handler.InventoryReserveHandler")
	defer span.End()

	fmt.Println("【责任链】=> 步骤2: 预占库存...")

	quantityStr := strconv.Itoa(orderCtx.Event.Quantity)
	if quantityStr == "0" {
		quantityStr = "1"
	}

	var reservedItems []string
	for _, item := range orderCtx.Event.Items {
		q := url.Values{}
		q.Set("itemId", item)
		q.Set("quantity", quantityStr)
		q.Set("userId", orderCtx.Event.UserID)
		q.Set("orderId", orderCtx.OrderId) // 传递订单ID
		if err := orderCtx.HTTPClient.Post(ctx, inventoryReserveURL, q); err != nil {
			span.RecordError(err)
			// ✨ [重大改变] 不再直接调用补偿，只是返回错误
			span.SetStatus(codes.Error, fmt.Sprintf("Inventory reservation failed for %s", item))
			return err
		}

		// ✨ [重大改变] 预占成功后，注册一个对应的补偿函数
		// 使用闭包来捕获当前 item 的值
		currentItem := item
		orderCtx.AddCompensation(func(ctx context.Context) {
			compCtx, compSpan := orderCtx.HTTPClient.Tracer.Start(ctx, "compensation.ReleaseStock")
			defer compSpan.End()

			compSpan.SetAttributes(attribute.String("item.id", currentItem))

			releaseParams := url.Values{
				"itemId":  {currentItem},
				"orderId": {orderCtx.OrderId},
			}
			// 在真实世界中，补偿失败需要有重试或告警机制
			if err := orderCtx.HTTPClient.Post(compCtx, inventoryReleaseURL, releaseParams); err != nil {
				compSpan.RecordError(err)
			}
		})
		reservedItems = append(reservedItems, item)
	}

	span.AddEvent("All Items reserved successfully", trace.WithAttributes(attribute.StringSlice("reserved_items", reservedItems)))

	return h.executeNext(orderCtx)
}
