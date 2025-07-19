package saga

import (
	"context"
	"go.opentelemetry.io/otel/trace"
	"nexus/internal/service/order/domain"
	"nexus/internal/service/order/domain/port"
	"sync"

	"github.com/rs/zerolog/log"
)

// OrderContext 在 Saga 流程中传递上下文数据。
// 【核心改造】: 所有外部依赖都变成了抽象接口。
type OrderContext struct {
	Ctx    context.Context
	Order  *domain.Order // <-- 传递核心领域对象
	Tracer trace.Tracer  // Tracer 依然需要

	// 依赖出站端口 (Interfaces)
	FraudService     port.FraudDetectionService
	InventoryService port.InventoryService
	PricingService   port.PricingService
	ShippingService  port.ShippingService
	Scheduler        port.DelayScheduler
	Notifier         port.NotificationProducer
	SeckillService   port.SeckillService

	// Saga 补偿逻辑保持不变
	compensations []func(ctx context.Context)
	compLock      sync.Mutex
}

// AddCompensation, TriggerCompensation 等方法保持不变...
func (c *OrderContext) AddCompensation(comp func(ctx context.Context)) {
	c.compLock.Lock()
	defer c.compLock.Unlock()
	c.compensations = append([]func(context.Context){comp}, c.compensations...)
}

func (c *OrderContext) TriggerCompensation(ctx context.Context) {
	c.compLock.Lock()
	defer c.compLock.Unlock()
	log.Printf("INFO: [Order: %s] Executing %d compensation functions.", c.Order.ID, len(c.compensations))
	for _, comp := range c.compensations {
		comp(ctx)
	}
}

// Handler 和 NextHandler 接口定义保持不变...
type Handler interface {
	SetNext(handler Handler) Handler
	Handle(orderCtx *OrderContext) error
}

type NextHandler struct {
	next Handler
}

func (h *NextHandler) SetNext(handler Handler) Handler {
	h.next = handler
	return handler
}

func (h *NextHandler) executeNext(orderCtx *OrderContext) error {
	if h.next != nil {
		return h.next.Handle(orderCtx)
	}
	return nil
}
