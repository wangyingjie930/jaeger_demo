package order

import (
	"fmt"
	"go.opentelemetry.io/otel/codes"
	"net/url"
	"strings"
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
	q := url.Values{}
	q.Set("items", strings.Join(orderCtx.Event.Items, ","))
	q.Set("userId", orderCtx.Event.UserID)
	if err := orderCtx.HTTPClient.Post(ctx, fraudDetectionServiceURL, q); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Fraud check failed")
		return err // 中断链
	}

	return h.executeNext(orderCtx)
}
