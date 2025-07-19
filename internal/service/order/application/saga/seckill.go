package saga

import (
	"fmt"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"nexus/internal/service/order/domain/port"
)

// SeckillHandler 负责秒杀资格校验步骤，并支持Saga补偿。
type SeckillHandler struct {
	NextHandler
}

func (h *SeckillHandler) Handle(orderCtx *OrderContext) error {
	productID := orderCtx.Order.SeckillProductID
	// 如果不是秒杀订单，直接跳到下一个处理器
	if productID == "" {
		return h.executeNext(orderCtx)
	}

	ctx, span := orderCtx.Tracer.Start(orderCtx.Ctx, "saga.SeckillCheck")
	defer span.End()

	span.SetAttributes(
		attribute.String("seckill.product.id", productID),
		attribute.String("user.id", orderCtx.Order.UserID),
	)

	fmt.Println("【Saga】=> 步骤 0.5: 秒杀资格校验...")

	// 调用抽象接口，而不是具体实现
	result, err := orderCtx.SeckillService.AttemptSeckill(ctx, productID, orderCtx.Order.UserID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Seckill service failed")
		return err // 中断Saga
	}

	switch result {
	case port.SeckillResultSuccess:
		fmt.Printf("【Saga】=> 用户 %s 成功抢到商品 %s\n", orderCtx.Order.UserID, productID)
		span.AddEvent("Seckill check passed")

		// 继续执行下一个处理器
		return h.executeNext(orderCtx)

	case port.SeckillResultSoldOut:
		err = port.ErrSeckillSoldOut
		span.SetStatus(codes.Error, err.Error())
		return err // 中断Saga

	case port.SeckillResultAlreadyPurchased:
		err = port.ErrSeckillAlreadyPurchased
		span.SetStatus(codes.Error, err.Error())
		return err // 中断Saga

	default:
		err = fmt.Errorf("unknown result from seckill service: %v", result)
		span.RecordError(err)
		span.SetStatus(codes.Error, "Unknown seckill error")
		return err // 中断Saga
	}
}
