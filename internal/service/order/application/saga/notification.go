package saga

import (
	"github.com/wangyingjie930/nexus-pkg/logger"
	"go.opentelemetry.io/otel/attribute"
)

// NotificationHandler 是 Saga 流程的最后一步，负责发送最终通知。
type NotificationHandler struct {
	NextHandler
}

func (h *NotificationHandler) Handle(orderCtx *OrderContext) error {
	ctx, span := orderCtx.Tracer.Start(orderCtx.Ctx, "saga.Notification")
	defer span.End()

	span.SetAttributes(
		attribute.String("messaging.system", "kafka"), // 这里的属性是描述性的，依然可以保留
		attribute.String("messaging.destination.topic", "notifications"),
	)

	logger.Ctx(ctx).Println("【Saga】=> 步骤 Final: 发送订单创建成功通知...")

	// 【核心改造】: 调用抽象的通知服务端口
	err := orderCtx.Notifier.SendOrderCreated(ctx, orderCtx.Order)

	// 【核心改造，维持原有逻辑】:
	// 严格遵循原始设计，发送通知失败是一个非关键路径的失败。
	// 我们只记录一个警告，然后让整个Saga流程成功结束。
	// 后续可以通过监控告警和后台任务来进行补偿。
	if err != nil {
		logger.Ctx(ctx).Error().Err(err).Str("order", orderCtx.Order.ID).Msg("WARN:Failed to publish notification")
		span.RecordError(err) // 在 tracing 中依然要记录这个非致命错误，以便排查
	}

	span.AddEvent("Saga process finalized and notification sent (or attempted).")

	// 由于这是链的末端，调用 executeNext 会返回 nil，代表整个流程成功结束。
	return h.executeNext(orderCtx)
}
