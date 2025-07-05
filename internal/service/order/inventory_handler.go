package order

import (
	"fmt"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"net/url"
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

	quantityStr := orderCtx.Request.URL.Query().Get("quantity")
	if quantityStr == "" {
		quantityStr = "1"
	}

	var reservedItems []string
	for _, item := range orderCtx.Items {
		q := url.Values{}
		q.Set("itemId", item)
		q.Set("quantity", quantityStr)
		q.Set("userId", orderCtx.UserId)
		q.Set("orderId", orderCtx.OrderId) // 传递订单ID
		if err := orderCtx.HTTPClient.Post(ctx, inventoryReserveURL, q); err != nil {
			span.RecordError(err)
			// ✨ [重大改变] 不再直接调用补偿，只是返回错误
			http.Error(orderCtx.Writer, fmt.Sprintf("Inventory reservation failed for %s", item), http.StatusInternalServerError)
			return err
		}

		// ✨ [重大改变] 预占成功后，注册一个对应的补偿函数
		// 使用闭包来捕获当前 item 的值
		currentItem := item
		orderCtx.AddCompensation(func() {
			compCtx, compSpan := orderCtx.HTTPClient.Tracer.Start(orderCtx.Ctx, "compensation.ReleaseStock")
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
