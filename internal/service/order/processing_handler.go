package order

import (
	"context"
	"encoding/json"
	"github.com/rs/zerolog/log"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"nexus/internal/pkg/httpclient"
	"nexus/internal/pkg/mq"
	"time"
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
	ctx, span := orderCtx.HTTPClient.Tracer.Start(orderCtx.Ctx, "handler.CreateOrderProcess")
	defer span.End()

	// todo: 创建订单

	// 设置"超时未支付"自动取消任务 (模拟延迟消息)
	log.Printf("INFO: [Order: %s] Setting up auto-cancellation task in %v.", orderCtx.OrderId, paymentTimeout)

	// 延迟队列 超时未支付取消订单
	sendTimeoutCheckTask(ctx, orderCtx)

	return h.executeNext(orderCtx)
}

// ✨ 新增: sendTimeoutCheckTask 负责发送延迟的超时检查任务
func sendTimeoutCheckTask(ctx context.Context, orderCtx *OrderContext) {
	_, span := orderCtx.HTTPClient.Tracer.Start(ctx, "order-service.SendTimeoutCheckTask")
	defer span.End()

	taskEvent := OrderTimeoutCheckEvent{
		TraceID:      trace.SpanFromContext(ctx).SpanContext().TraceID().String(),
		OrderID:      orderCtx.OrderId,
		UserID:       orderCtx.Event.UserID,
		Items:        orderCtx.Event.Items,
		CreationTime: time.Now(),
	}
	taskBytes, _ := json.Marshal(taskEvent)

	delayTimestamp := time.Now().Add(5 * time.Second).Format(time.RFC3339)

	msg := kafka.Message{
		Key:   []byte(orderCtx.OrderId),
		Value: taskBytes,
		Headers: []kafka.Header{
			{Key: "real-topic", Value: []byte(orderCtx.KafkaDelayRealTopic)},
			{Key: "delay-timestamp", Value: []byte(delayTimestamp)},
		},
	}
	mq.InjectTraceContext(ctx, &msg.Headers)

	if err := orderCtx.KafkaDelayWriters["delay_topic_5s"].WriteMessages(ctx, msg); err != nil {
		log.Printf("ERROR: [Order: %s] Failed to send timeout check task: %v", orderCtx.OrderId, err)
		span.RecordError(err)
	} else {
		log.Printf("INFO: [Order: %s] Sent timeout check task, will be checked around %s.", orderCtx.OrderId, delayTimestamp)
		span.AddEvent("Timeout check task sent to delay queue.")
	}
}

// ✨ 新增: processTimeoutCheckMessage 处理到期的订单检查任务
func ProcessTimeoutCheckMessage(msg kafka.Message, httpClient *httpclient.Client) {
	var event OrderTimeoutCheckEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		log.Printf("ERROR: Failed to unmarshal timeout event: %v", err)
		return
	}

	parentCtx := mq.ExtractTraceContext(context.Background(), msg.Headers)
	ctx, span := httpClient.Tracer.Start(parentCtx, "order-service.ProcessTimeoutCheck", trace.WithSpanKind(trace.SpanKindConsumer))
	defer span.End()

	span.SetAttributes(
		attribute.String("order.id", event.OrderID),
		attribute.String("user.id", event.UserID),
	)
	log.Printf("INFO: [Order: %s] Timeout checker running.", event.OrderID)

	currentStatus := StatePaid
	if true {
		currentStatus = StatePendingPayment
	}

	span.SetAttributes(
		attribute.String("currentStatus", string(currentStatus)),
		attribute.String("orderId", event.OrderID),
	)

	log.Printf("INFO: [Order: %s] Timeout checker running. Current status is '%s'.", event.OrderID, currentStatus)

	// 1. 从当前带有超时的上下文中，提取出纯粹的、不含超时的 Span 上下文信息。
	//    这部分信息只包含 TraceID, SpanID 等，用于关联链路。
	spanContext := trace.SpanContextFromContext(ctx)

	// 2. 创建一个新的、完全独立的后台上下文。
	detachedCtx := context.Background()

	// 3. 将之前提取的 Span 上下文信息“注入”到这个新的后台上下文中。
	//    这样，我们就得到了一个既能关联上级链路，又没有超时限制的新上下文。
	timeoutTaskCtx := trace.ContextWithRemoteSpanContext(detachedCtx, spanContext)

	if currentStatus == StatePendingPayment {
		log.Printf("WARN: [Order: %s] Order has not been paid within the time limit. Cancelling and releasing resources.", event.OrderID)

		//orderCtx.TriggerCompensation(timeoutTaskCtx) todo: 回滚
		new(InventoryReserveHandler).RollbackAll(timeoutTaskCtx, httpClient, event.OrderID, event.Items)
		span.AddEvent("TriggerCompensation")

		// (可选) 发送一个订单因超时被取消的通知给用户
		// orderCtx.TriggerNotification(orderSvc.StateCancelled)
	}
}
