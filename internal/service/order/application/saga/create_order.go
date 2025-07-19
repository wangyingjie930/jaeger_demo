package saga

import (
	"fmt"
	"nexus/internal/service/order/domain"
	"time"
)

// CreateOrderHandler 负责持久化订单并调度后续任务。
type CreateOrderHandler struct {
	NextHandler
	repo domain.OrderRepository // <-- 注入仓储接口
}

func NewCreateOrderHandler(repo domain.OrderRepository) *CreateOrderHandler {
	return &CreateOrderHandler{repo: repo}
}

func (h *CreateOrderHandler) Handle(orderCtx *OrderContext) error {
	ctx, span := orderCtx.Tracer.Start(orderCtx.Ctx, "saga.CreateOrder")
	defer span.End()

	fmt.Println("【Saga】=> 步骤 4: 创建订单实体并调度支付超时任务...")

	// 1. 将订单状态更新为待支付并持久化
	orderCtx.Order.MarkAsPendingPayment()
	if err := h.repo.Save(ctx, orderCtx.Order); err != nil {
		return fmt.Errorf("failed to save pending payment order: %w", err)
	}
	span.AddEvent("Pending payment order saved to DB.")

	// 2. 【核心改造】: 调用抽象的调度器接口
	err := orderCtx.Scheduler.SchedulePaymentTimeout(
		ctx,
		orderCtx.Order.ID,
		orderCtx.Order.UserID,
		orderCtx.Order.Items,
		time.Now(),
	)
	if err != nil {
		// 发送调度任务失败是一个需要关注的错误，但通常不应让主流程失败。
		// 因为订单已经创建成功，可以后续补偿。这里只记录错误。
		span.RecordError(err)
		// log.Printf("ERROR: [Order: %s] Failed to schedule payment timeout: %v", orderCtx.Order.ID, err)
	}

	return h.executeNext(orderCtx)
}
