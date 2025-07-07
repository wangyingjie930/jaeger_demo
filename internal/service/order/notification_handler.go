package order

import (
	"encoding/json"
	"fmt"
	"jaeger-demo/internal/pkg/mq"
	"log"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

var (
	paymentTimeout = 3 * time.Second // 支付时限，生产环境应可配置
)

// NotificationHandler 是责任链的最后一环，负责发送最终通知
type NotificationHandler struct {
	NextHandler
}

func (h *NotificationHandler) Handle(orderCtx *OrderContext) error {
	// 从上下文中获取 Tracer，创建业务 Span
	ctx, span := orderCtx.HTTPClient.Tracer.Start(orderCtx.Ctx, "handler.Notification")
	defer span.End()

	span.SetAttributes(
		attribute.String("messaging.system", "kafka"),
		attribute.String("messaging.destination", orderCtx.KafkaWriter.Topic),
	)

	fmt.Println("【责任链】=> 步骤 Final: 发送订单创建成功通知...")

	// 准备消息内容
	message := fmt.Sprintf(
		"Your order %s is waiting for payment. Please complete it within %v.",
		orderCtx.OrderId, paymentTimeout,
	)
	if orderCtx.Event.PromoId != "" {
		message = fmt.Sprintf("Your VIP promotion order (%s) has been successfully created!", orderCtx.Event.PromoId)
	}

	event := NotificationEvent{
		UserID:      orderCtx.Event.UserID,
		Message:     message,
		PromotionID: orderCtx.Event.PromoId,
	}
	eventBytes, err := json.Marshal(event)
	if err != nil {
		// 这是一个严重的本地错误，应该记录并可能中断流程
		err = fmt.Errorf("failed to marshal notification event: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		// 虽然通知不是最关键步骤，但序列化失败是程序bug，应该返回错误
		//http.Error(orderCtx.Writer, err.Error(), http.StatusInternalServerError)
		return err
	}

	// 2. 调用 mq 包提供的通用方法来发送消息
	err = mq.ProduceMessage(ctx, orderCtx.KafkaWriter, []byte(orderCtx.Event.UserID), eventBytes)
	if err != nil {
		// ✨ 关键点：发送通知失败，通常不应触发回滚！
		// 这是一个非关键路径的失败，主订单流程已经成功。
		// 我们应该只记录一个警告，然后让整个流程成功结束。
		// 后续可以通过监控告警和后台任务来进行补偿。
		log.Printf("WARN: Failed to publish notification event for order %s: %v", orderCtx.OrderId, err)
		span.RecordError(err) // 在 tracing 中记录这个非致命错误
	}

	// 3. 成功结束整个HTTP请求
	span.AddEvent("Complex order finalized and notification sent (or attempted).")
	//orderCtx.Writer.WriteHeader(http.StatusOK)
	//orderCtx.Writer.Write([]byte("Complex order created successfully! (Processed by Chain of Responsibility)"))

	// 由于这是链的末端，调用 executeNext 会返回 nil
	return h.executeNext(orderCtx)
}
