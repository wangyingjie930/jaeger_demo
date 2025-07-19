package application

import (
	"fmt"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

type SeckillHandler struct {
	NextHandler
	seckillService *SeckillService // ✨ [新增] 持有 SeckillService 实例
}

// NewSeckillHandler 创建一个新的 SeckillHandler
func NewSeckillHandler(seckillService *SeckillService) *SeckillHandler {
	return &SeckillHandler{
		seckillService: seckillService,
	}
}

func (h *SeckillHandler) Handle(orderCtx *OrderContext) error {
	productID := orderCtx.Event.SeckillProductID
	if productID == "" {
		return h.executeNext(orderCtx)
	}

	ctx, span := orderCtx.HTTPClient.Tracer.Start(orderCtx.Ctx, "handler.SeckillHandler")
	defer span.End()

	span.SetAttributes(
		attribute.String("seckill.product.id", productID),
		attribute.String("user.id", orderCtx.Event.UserID),
	)

	fmt.Println("【责任链】=> 步骤 0.5: 秒杀资格校验 (调用 SeckillService)...")

	// ✨ [核心改造] 调用 SeckillService 处理业务逻辑
	resultCode, err := h.seckillService.AttemptSeckill(ctx, productID, orderCtx.Event.UserID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Seckill service failed")
		//http.Error(orderCtx.Writer, "秒杀系统繁忙，请稍后再试", http.StatusInternalServerError)
		return err // 中断链
	}

	switch resultCode {
	case ResultSuccess:
		span.AddEvent("Seckill successful, proceeding to next handler.")
		fmt.Println("【秒杀】=> 用户", orderCtx.Event.UserID, "成功抢到商品", productID)
		return h.executeNext(orderCtx)
	case ResultSoldOut:
		span.AddEvent("Seckill failed: product sold out.")
		span.SetStatus(codes.Error, "Product sold out")
		//http.Error(orderCtx.Writer, "抱歉，商品已售罄", http.StatusForbidden)
		return fmt.Errorf("product %s is sold out", productID)
	case ResultAlreadyPurchased:
		span.AddEvent("Seckill failed: user already purchased.")
		span.SetStatus(codes.Error, "User already purchased")
		//http.Error(orderCtx.Writer, "您已购买过此商品，请勿重复下单", http.StatusForbidden)
		return fmt.Errorf("user %s already purchased product %s", orderCtx.Event.UserID, productID)
	default:
		unknownErr := fmt.Errorf("unknown result code from seckill service: %d", resultCode)
		span.RecordError(unknownErr)
		span.SetStatus(codes.Error, "Unknown seckill error")
		//http.Error(orderCtx.Writer, "秒杀失败，未知错误", http.StatusInternalServerError)
		return unknownErr
	}
}
