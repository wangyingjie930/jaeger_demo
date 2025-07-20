package saga

import (
	"go.opentelemetry.io/otel/codes"
	"nexus/internal/pkg/logger"
)

// FraudCheckHandler 负责欺诈检测步骤。
type FraudCheckHandler struct {
	NextHandler
}

func (h *FraudCheckHandler) Handle(orderCtx *OrderContext) error {
	ctx, span := orderCtx.Tracer.Start(orderCtx.Ctx, "saga.FraudCheck")
	defer span.End()

	logger.Ctx(ctx).Println("【Saga】=> 步骤 1: 欺诈检测...")

	// 【核心改造】: 调用抽象接口，而不是具体的HTTP客户端。
	if err := orderCtx.FraudService.CheckFraud(ctx, orderCtx.Order); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Fraud check failed")
		return err // 中断 Saga
	}

	return h.executeNext(orderCtx)
}
