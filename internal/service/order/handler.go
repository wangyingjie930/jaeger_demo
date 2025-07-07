package order

import (
	"context"
	"github.com/rs/zerolog/log"
	"github.com/segmentio/kafka-go"
	"jaeger-demo/internal/pkg/httpclient"
	"os"
	"sync"
)

// OrderContext 用于在链中传递订单处理所需的所有数据
type OrderContext struct {
	HTTPClient *httpclient.Client // ✨ [修改] 注入 HTTPClient
	Ctx        context.Context
	OrderId    string

	// ✨ [新增] 补偿函数栈
	compensations []CompensationFunc
	// ✨ [新增] 用于保护补偿栈并发安全的锁
	compLock sync.Mutex

	KafkaWriter *kafka.Writer // ✨ [新增] 注入 Kafka 生产者
	// ✨ 新增：保存从Kafka接收到的原始事件
	Event *OrderCreationEvent
}

// CompensationFunc 定义了补偿操作的函数签名
type CompensationFunc func(ctx context.Context)

// AddCompensation ✨ [新增] AddCompensation 将一个补偿函数推入栈中
func (c *OrderContext) AddCompensation(comp CompensationFunc) {
	c.compLock.Lock()
	defer c.compLock.Unlock()
	// 使用 LIFO (后进先出) 方式，后注册的补偿先执行
	c.compensations = append([]CompensationFunc{comp}, c.compensations...)
}

// TriggerCompensation 负责执行所有已注册的SAGA补偿函数
func (c *OrderContext) TriggerCompensation(ctx context.Context) {
	c.compLock.Lock()
	defer c.compLock.Unlock()

	log.Printf("INFO: [Order: %s] Executing %d compensation functions.", c.OrderId, len(c.compensations))
	for _, comp := range c.compensations {
		comp(ctx)
	}
}

// Handler 定义了责任链中每个节点的接口
type Handler interface {
	// SetNext 设置链中的下一个处理器
	SetNext(handler Handler) Handler
	// Handle 执行当前节点的处理逻辑
	Handle(orderCtx *OrderContext) error
}

// NextHandler 是一个辅助结构，可以嵌入到具体的处理器中，以减少重复代码
type NextHandler struct {
	next Handler
}

func (h *NextHandler) SetNext(handler Handler) Handler {
	h.next = handler
	return handler
}

// executeNext 封装了调用下一个处理器的通用逻辑
func (h *NextHandler) executeNext(orderCtx *OrderContext) error {
	if h.next != nil {
		return h.next.Handle(orderCtx)
	}
	return nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
