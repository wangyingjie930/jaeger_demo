package order

import (
	"context"
	"math/rand"
	"time"

	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/attribute"
)

// mapCarrier 实现了 propagation.TextMapCarrier 接口，用于在内存中传递追踪上下文
type mapCarrier struct {
	data map[string]string
}

func (c *mapCarrier) Get(key string) string {
	return c.data[key]
}

func (c *mapCarrier) Set(key, value string) {
	c.data[key] = value
}

func (c *mapCarrier) Keys() []string {
	keys := make([]string, 0, len(c.data))
	for k := range c.data {
		keys = append(keys, k)
	}
	return keys
}

type ProcessHandler struct {
	NextHandler
}

func (h *ProcessHandler) Handle(orderCtx *OrderContext) error {
	_, span := orderCtx.HTTPClient.Tracer.Start(orderCtx.Ctx, "handler.CreateOrderProcess")
	defer span.End()

	// todo: 创建订单

	// 设置"超时未支付"自动取消任务 (模拟延迟消息)
	log.Printf("INFO: [Order: %s] Setting up auto-cancellation task in %v.", orderCtx.OrderId, paymentTimeout)
	time.AfterFunc(paymentTimeout, func() {
		checkOrderTimeout(orderCtx)
	})

	return h.executeNext(orderCtx)
}

// checkOrderTimeout 模拟延迟队列的消费者，用于处理订单支付超时
func checkOrderTimeout(orderCtx *OrderContext) {
	// 实际业务中，我们会从数据库查询订单的最新状态
	// currentStatus := database.GetOrderStatus(orderCtx.OrderId)
	// 此处我们模拟一个场景：订单依然是"待支付"状态
	_, span := orderCtx.HTTPClient.Tracer.Start(orderCtx.Ctx, "checkOrderTimeout")
	defer span.End()

	currentStatus := StatePaid
	if rand.Intn(1) == 0 {
		currentStatus = StatePendingPayment
	}

	span.SetAttributes(
		attribute.String("currentStatus", string(currentStatus)),
		attribute.String("orderId", orderCtx.OrderId),
	)

	log.Printf("INFO: [Order: %s] Timeout checker running. Current status is '%s'.", orderCtx.OrderId, currentStatus)

	if currentStatus == StatePendingPayment {
		log.Printf("WARN: [Order: %s] Order has not been paid within the time limit. Cancelling and releasing resources.", orderCtx.OrderId)

		orderCtx.TriggerCompensation(context.Background())
		span.AddEvent("TriggerCompensation")

		// (可选) 发送一个订单因超时被取消的通知给用户
		// orderCtx.TriggerNotification(orderSvc.StateCancelled)
	}
}
