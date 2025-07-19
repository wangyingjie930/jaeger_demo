// internal/service/order/application/service.go
package application

import (
	"context"
	"go.opentelemetry.io/otel/attribute"
	"log"
	"nexus/internal/service/order/application/saga"
	"nexus/internal/service/order/domain"
	"nexus/internal/service/order/domain/port"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// OrderApplicationService 现在只关注业务流程编排。
type OrderApplicationService struct {
	orderRepo         domain.OrderRepository
	processingTimeout time.Duration
	tracer            trace.Tracer

	createOrderProducer domain.OrderProducer

	fraudService     port.FraudDetectionService
	inventoryService port.InventoryService
	pricingService   port.PricingService
	shippingService  port.ShippingService
	scheduler        port.DelayScheduler
	notifier         port.NotificationProducer
	seckillService   port.SeckillService
}

func NewOrderApplicationService(orderRepo domain.OrderRepository, processingTimeout time.Duration, tracer trace.Tracer, createOrderProducer domain.OrderProducer, fraudService port.FraudDetectionService, inventoryService port.InventoryService, pricingService port.PricingService, shippingService port.ShippingService, scheduler port.DelayScheduler, notifier port.NotificationProducer, seckillService port.SeckillService) *OrderApplicationService {
	return &OrderApplicationService{
		orderRepo: orderRepo, processingTimeout: processingTimeout,
		tracer: tracer, createOrderProducer: createOrderProducer,
		fraudService: fraudService, inventoryService: inventoryService,
		pricingService: pricingService, shippingService: shippingService,
		scheduler: scheduler, notifier: notifier, seckillService: seckillService}
}

// HandleOrderCreationEvent 是新的、被动的业务处理入口。
// 它由驱动适配器（如KafkaConsumerAdapter或HttpHandler）调用。
func (s *OrderApplicationService) HandleOrderCreationEvent(ctx context.Context, event *domain.OrderCreationRequested) error {
	ctx, span := s.tracer.Start(ctx, "app.HandleOrderCreationEvent", trace.WithSpanKind(trace.SpanKindConsumer))
	defer span.End()

	// 2. 为每个订单的处理流程设置独立的超时时间
	processingCtx, cancel := context.WithTimeout(ctx, s.processingTimeout)
	defer cancel()

	// 3. 使用领域工厂函数创建订单实体
	orderEntity, err := domain.NewOrder(event)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to create order entity")
		log.Printf("ERROR: [Order: %s] Failed to create order entity: %v", event.EventID, err)
		return err
	}

	// 4. 【可选】初始持久化
	if err := s.orderRepo.Save(processingCtx, orderEntity); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to save initial order")
		log.Printf("ERROR: [Order: %s] Failed to save initial order: %v", orderEntity.ID, err)
		return err
	}
	span.AddEvent("Initial order saved with CREATED state.")

	// 5. 构造责任链所需的上下文（OrderContext）
	orderContext := &saga.OrderContext{
		Ctx:              processingCtx,
		Order:            orderEntity, // 传递实体
		Tracer:           s.tracer,    // 传递Tracer
		FraudService:     s.fraudService,
		InventoryService: s.inventoryService,
		PricingService:   s.pricingService,
		ShippingService:  s.shippingService,
		Scheduler:        s.scheduler,
		Notifier:         s.notifier,
		SeckillService:   s.seckillService,
	}

	log.Printf("INFO: [Order: %s] Starting verification and reservation process for user %s.", orderEntity.ID, event.UserID)

	orderChain := s.buildChain()

	// 6. 执行责任链
	if err := orderChain.Handle(orderContext); err != nil {
		log.Printf("ERROR: [Order: %s] Order processing chain failed: %v. SAGA compensation triggered.", orderEntity.ID, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "Order processing failed in chain")

		orderEntity.MarkAsFailed()
		if updateErr := s.orderRepo.Save(processingCtx, orderEntity); updateErr != nil {
			log.Printf("CRITICAL: [Order: %s] Failed to update order status to FAILED after compensation: %v", orderEntity.ID, updateErr)
			span.RecordError(updateErr, trace.WithAttributes(attribute.Bool("critical.error", true)))
		}
		return err // 返回主错误
	}

	// 7. 流程成功，更新订单状态
	orderEntity.MarkAsPendingPayment()
	if err := s.orderRepo.Save(processingCtx, orderEntity); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to save order as pending payment")
		log.Printf("CRITICAL: [Order: %s] Failed to save order as PENDING_PAYMENT: %v", orderEntity.ID, err)
		orderContext.TriggerCompensation(processingCtx) // 触发补偿
		return err
	}

	log.Printf("SUCCESS: [Order: %s] All resources reserved. Order status is now PENDING_PAYMENT.", orderEntity.ID)
	span.AddEvent("Order successfully processed and is pending payment.")
	return nil
}

// RequestOrderCreation 是暴露给接口层（如HTTP Handler）的入口方法
// 在当前的异步架构中，此方法的主要职责是发送一个创建订单的消息到 Kafka
func (s *OrderApplicationService) RequestOrderCreation(ctx context.Context, req *CreateOrderRequest) (*CreateOrderResponse, error) {
	ctx, span := s.tracer.Start(ctx, "app.RequestOrderCreation")
	defer span.End()

	// 1. 生成唯一的事件ID/订单ID
	eventID := uuid.New().String()
	req.EventID = eventID
	req.TraceID = span.SpanContext().TraceID().String()

	// 2. 将应用层 DTO 转换为领域事件
	event := req.ToOrderCreationEvent()

	if err := s.createOrderProducer.Product(ctx, event); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to notify product")
		return nil, err
	}

	span.AddEvent("Order creation request sent to Kafka queue.")
	log.Printf("Successfully enqueued order creation request for user %s with EventID %s", event.UserID, event.EventID)

	// 4. 立即返回响应，告知客户端请求已被接受
	return &CreateOrderResponse{
		OrderID: eventID,
		Status:  domain.StateCreated, // 返回一个初始状态
		Message: "Your order is being processed.",
	}, nil
}

// 新增: processTimeoutCheckMessage 处理到期的订单检查任务
func (s *OrderApplicationService) ProcessTimeoutCheckMessage(ctx context.Context, event *domain.OrderTimeoutCheckEvent) error {
	ctx, span := s.tracer.Start(ctx, "order-service.ProcessTimeoutCheck", trace.WithSpanKind(trace.SpanKindConsumer))
	defer span.End()

	span.SetAttributes(
		attribute.String("order.id", event.OrderID),
		attribute.String("user.id", event.UserID),
	)
	log.Printf("INFO: [Order: %s] Timeout checker running.", event.OrderID)

	currentStatus := domain.StatePaid
	if true {
		currentStatus = domain.StatePendingPayment
	}

	span.SetAttributes(
		attribute.String("currentStatus", string(currentStatus)),
		attribute.String("orderId", event.OrderID),
	)

	log.Printf("INFO: [Order: %s] Timeout checker running. Current status is '%s'.", event.OrderID, currentStatus)

	// 1. 从当前带有超时的上下文中，提取出纯粹的、不含超时的 Span 上下文信息。
	//    这部分信息只包含 TraceID, SpanID 等，用于关联链路。
	spanContext := trace.SpanContextFromContext(ctx)

	// 3. 将之前提取的 Span 上下文信息“注入”到这个新的后台上下文中。
	//    这样，我们就得到了一个既能关联上级链路，又没有超时限制的新上下文。
	timeoutTaskCtx := trace.ContextWithRemoteSpanContext(context.Background(), spanContext)

	if currentStatus == domain.StatePendingPayment {
		log.Printf("WARN: [Order: %s] Order has not been paid within the time limit. Cancelling and releasing resources.", event.OrderID)

		itemsToReserve := map[string]int{}
		for _, item := range event.Items {
			itemsToReserve[item] = 1
		}

		s.inventoryService.ReleaseStock(timeoutTaskCtx, event.OrderID, itemsToReserve)
		span.AddEvent("TriggerCompensation")

		// (可选) 发送一个订单因超时被取消的通知给用户
		// orderCtx.TriggerNotification(orderSvc.StateCancelled)
	}
	return nil
}

func (s *OrderApplicationService) buildChain() saga.Handler {
	orderProcessingChain := new(saga.FraudCheckHandler)
	orderProcessingChain.
		SetNext(new(saga.InventoryHandler)).
		SetNext(new(saga.PricingHandler)).
		SetNext(new(saga.SeckillHandler)).
		SetNext(saga.NewCreateOrderHandler(s.orderRepo)).
		SetNext(new(saga.NotificationHandler))

	return orderProcessingChain
}
