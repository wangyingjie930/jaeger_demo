package order

import (
	"fmt"
	"net/http"
)

type FraudCheckHandler struct {
	NextHandler
}

var (
	fraudDetectionServiceURL = getEnv("FRAUD_DETECTION_SERVICE_URL", "http://localhost:8085/check")
)

func (h *FraudCheckHandler) Handle(orderCtx *OrderContext) error {
	ctx, span := orderCtx.HTTPClient.Tracer.Start(orderCtx.Ctx, "handler.FraudCheck")
	defer span.End()

	fmt.Println("【责任链】=> 步骤1: 欺诈检测...")
	if err := orderCtx.HTTPClient.Post(ctx, fraudDetectionServiceURL, orderCtx.Params); err != nil {
		span.RecordError(err)
		http.Error(orderCtx.Writer, "Fraud check failed", http.StatusBadRequest)
		return err // 中断链
	}

	return h.executeNext(orderCtx)
}
