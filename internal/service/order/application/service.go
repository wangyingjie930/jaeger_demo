// internal/service/order/application/service.go
package application

import (
	"context"
	"encoding/json"
	"go.opentelemetry.io/otel/attribute"
	"log"
	"nexus/internal/pkg/httpclient"
	"nexus/internal/pkg/mq"
	"nexus/internal/service/order/application/saga"
	"nexus/internal/service/order/domain"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// OrderApplicationService 是订单应用服务的实现
// 它封装了订单创建、查询等核心用例
type OrderApplicationService struct {
	orderRepo           domain.OrderRepository   // 订单仓储，用于持久化
	orderChain          saga.Handler             // 订单处理责任链的起点
	httpClient          *httpclient.Client       // HTTP 客户端，用于服务间调用
	notificationWriter  *kafka.Writer            // Kafka 生产者，用于发送通知
	delayWriters        map[string]*kafka.Writer // 延迟消息生产者
	timeoutCheckTopic   string                   // 超时检查的真实 Topic
	processingTimeout   time.Duration            // 订单处理的超时时间
	tracer              trace.Tracer             // OpenTelemetry Tracer
	orderCreationReader *kafka.Reader            // Kafka 消费者，用于消费订单创建请求
	wg                  sync.WaitGroup           // 用于管理消费者 Goroutine
}

// NewOrderApplicationService 创建一个新的订单应用服务实例
// 这是依赖注入（DI）发生的地方
func NewOrderApplicationService(
	repo domain.OrderRepository,
	chain saga.Handler,
	httpClient *httpclient.Client,
	notificationWriter *kafka.Writer,
	delayWriters map[string]*kafka.Writer,
	timeoutCheckTopic string,
	processingTimeout time.Duration,
	orderCreationReader *kafka.Reader,
) *OrderApplicationService {
	return &OrderApplicationService{
		orderRepo:           repo,
		orderChain:          chain,
		httpClient:          httpClient,
		notificationWriter:  notificationWriter,
		delayWriters:        delayWriters,
		timeoutCheckTopic:   timeoutCheckTopic,
		processingTimeout:   processingTimeout,
		tracer:              otel.Tracer("order-application-service"), // 创建 Tracer
		orderCreationReader: orderCreationReader,
	}
}

// StartConsuming 启动 Kafka 消费者来处理订单创建请求
// 这是一个长期运行的方法，应该在一个单独的 Goroutine 中启动
func (s *OrderApplicationService) StartConsuming(ctx context.Context) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		log.Println("✅ Order Application Service: Kafka Consumer started.")
		for {
			select {
			case <-ctx.Done(): // 监听上下文取消信号
				log.Println("🛑 Order Application Service: Kafka Consumer shutting down.")
				return
			default:
				msg, err := s.orderCreationReader.ReadMessage(ctx)
				if err != nil {
					// 如果上下文被取消，会产生一个错误，我们直接返回
					if ctx.Err() != nil {
						return
					}
					log.Printf("ERROR: could not read message from Kafka: %v. Retrying...", err)
					time.Sleep(5 * time.Second) // 避免无休止的快速失败循环
					continue
				}
				// 为每个消息启动一个独立的 Goroutine 进行处理
				go s.processOrderMessage(msg)
			}
		}
	}()
}

// StopConsuming 优雅地停止消费者
func (s *OrderApplicationService) StopConsuming() {
	s.orderCreationReader.Close()
	s.wg.Wait()
	log.Println("✅ Order Application Service: Kafka Consumer stopped.")
}

// processOrderMessage 是处理从 Kafka 收到的单个订单创建消息的核心方法
func (s *OrderApplicationService) processOrderMessage(msg kafka.Message) {
	// 1. 解析消息体
	var event domain.OrderCreationRequested
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		log.Printf("ERROR: Failed to unmarshal event: %v. Message will be skipped.", err)
		// 在生产环境中，应将消息移至死信队列（DLQ）
		return
	}

	// 2. 重建追踪上下文
	propagator := otel.GetTextMapPropagator()
	header := mq.KafkaHeaderCarrier(msg.Headers)
	parentCtx := propagator.Extract(context.Background(), &header)
	ctx, span := s.tracer.Start(parentCtx, "app.ProcessOrderMessage", trace.WithSpanKind(trace.SpanKindConsumer))
	defer span.End()

	// 3. 为每个订单的处理流程设置独立的超时时间
	processingCtx, cancel := context.WithTimeout(ctx, s.processingTimeout)
	defer cancel()

	// 4. 使用领域工厂函数创建订单实体
	orderEntity, err := domain.NewOrder(&event)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to create order entity")
		log.Printf("ERROR: [Order: %s] Failed to create order entity: %v", event.EventID, err)
		return
	}

	// 5. 【可选】初始持久化，将订单以 "CREATED" 状态保存
	// 这有助于追踪所有被系统接收到的请求，即使它们后来处理失败
	if err := s.orderRepo.Save(processingCtx, orderEntity); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to save initial order")
		log.Printf("ERROR: [Order: %s] Failed to save initial order: %v", orderEntity.ID, err)
		return
	}
	span.AddEvent("Initial order saved with CREATED state.")

	// 6. 构造责任链所需的上下文（OrderContext）
	orderContext := &saga.OrderContext{
		HTTPClient:          s.httpClient,
		KafkaNotifyWriter:   s.notificationWriter,
		KafkaDelayWriters:   s.delayWriters,
		KafkaDelayRealTopic: s.timeoutCheckTopic,
		Ctx:                 processingCtx,
		OrderId:             orderEntity.ID,
		Event:               ToCreateOrderRequest(&event).ToOrderCreationEvent(), // 复用DTO转换
	}

	log.Printf("INFO: [Order: %s] Starting verification and reservation process for user %s.", orderContext.OrderId, orderContext.Event.UserID)

	// 7. 【核心】执行责任链，驱动业务流程
	if err := s.orderChain.Handle(orderContext); err != nil {
		log.Printf("ERROR: [Order: %s] Order processing chain failed: %v. SAGA compensation triggered.", orderContext.OrderId, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "Order processing failed in chain")

		// 责任链内部的 TransactionHandler 会自动触发补偿
		// 应用层只需更新订单最终状态为 FAILED
		orderEntity.MarkAsFailed()
		if updateErr := s.orderRepo.Save(processingCtx, orderEntity); updateErr != nil {
			log.Printf("CRITICAL: [Order: %s] Failed to update order status to FAILED after compensation: %v", orderEntity.ID, updateErr)
			span.RecordError(updateErr, trace.WithAttributes(attribute.Bool("critical.error", true)))
		}
		return
	}

	// 8. 流程成功，更新订单状态为 "PENDING_PAYMENT"
	if err := orderEntity.MarkAsPendingPayment(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to mark order as pending payment")
		log.Printf("ERROR: [Order: %s] Failed to mark as PENDING_PAYMENT: %v", orderEntity.ID, err)
		// 理论上这里不应该失败，但如果失败，也应该触发补偿
		orderContext.TriggerCompensation(processingCtx)
		return
	}

	if err := s.orderRepo.Save(processingCtx, orderEntity); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to save order as pending payment")
		log.Printf("CRITICAL: [Order: %s] Failed to save order as PENDING_PAYMENT: %v", orderEntity.ID, err)
		// 这是流程最后一步的数据库失败，补偿是必须的
		orderContext.TriggerCompensation(processingCtx)
		return
	}

	log.Printf("SUCCESS: [Order: %s] All resources reserved. Order status is now PENDING_PAYMENT.", orderContext.OrderId)
	span.AddEvent("Order successfully processed and is pending payment.")
}

// CreateOrder 是暴露给接口层（如HTTP Handler）的入口方法
// 在当前的异步架构中，此方法的主要职责是发送一个创建订单的消息到 Kafka
func (s *OrderApplicationService) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*CreateOrderResponse, error) {
	ctx, span := s.tracer.Start(ctx, "app.CreateOrder")
	defer span.End()

	// 1. 生成唯一的事件ID/订单ID
	eventID := uuid.New().String()
	req.EventID = eventID
	req.TraceID = span.SpanContext().TraceID().String()

	// 2. 将应用层 DTO 转换为领域事件
	event := req.ToOrderCreationEvent()

	eventBytes, err := json.Marshal(event)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to marshal order creation event")
		return nil, err
	}

	// 3. 发送消息到 Kafka
	// 这里使用了一个临时的 writer, 更好的方式是从 service 中获取
	// 为了示例清晰，我们假设有一个全局的 writer
	// err = mq.ProduceMessage(ctx, s.orderCreationWriter, []byte(event.UserID), eventBytes)
	// if err != nil {
	// 	span.RecordError(err)
	// 	span.SetStatus(codes.Error, "Failed to produce message to Kafka")
	// 	return nil, err
	// }

	span.AddEvent("Order creation request sent to Kafka queue.")
	log.Printf("Successfully enqueued order creation request for user %s with EventID %s", event.UserID, event.EventID)

	// 4. 立即返回响应，告知客户端请求已被接受
	return &CreateOrderResponse{
		OrderID: eventID,
		Status:  domain.StateCreated, // 返回一个初始状态
		Message: "Your order is being processed.",
	}, nil
}
