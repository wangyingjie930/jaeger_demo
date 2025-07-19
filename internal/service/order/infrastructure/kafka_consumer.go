// internal/service/order/infrastructure/adapter/kafka_consumer_adapter.go
package infrastructure

import (
	"context"
	"encoding/json"
	"go.opentelemetry.io/otel"
	"log"
	"nexus/internal/pkg/mq"
	"nexus/internal/service/order/application"
	"nexus/internal/service/order/domain"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
)

// OrderConsumerAdapter 是一个驱动适配器，它监听Kafka消息并驱动应用服务。
type OrderConsumerAdapter struct {
	reader  *kafka.Reader
	appSvc  *application.OrderApplicationService // <-- 依赖应用服务层的接口
	wg      sync.WaitGroup
	stopped bool
}

// NewOrderConsumerAdapter 创建一个新的Kafka消费者适配器。
func NewOrderConsumerAdapter(reader *kafka.Reader, appSvc *application.OrderApplicationService) *OrderConsumerAdapter {
	return &OrderConsumerAdapter{
		reader: reader,
		appSvc: appSvc,
	}
}

// Start 开始监听Kafka主题。这是一个长期运行的方法。
func (a *OrderConsumerAdapter) Start(ctx context.Context) {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		log.Printf("✅ Kafka Consumer Adapter started for topic '%s'.", a.reader.Config().Topic)
		for {
			if a.stopped {
				return
			}
			// 我们使用FetchMessage而不是ReadMessage，以便更好地控制退出逻辑
			msg, err := a.reader.FetchMessage(ctx)
			if err != nil {
				// 如果是上下文取消导致的错误，则正常退出
				if ctx.Err() != nil {
					log.Println("🛑 Kafka Consumer Adapter shutting down.")
					return
				}
				log.Printf("ERROR: could not read message: %v. Retrying...", err)
				time.Sleep(1 * time.Second) // 避免快速失败循环
				continue
			}

			// 将具体的消息处理逻辑委托给一个私有方法
			a.processMessage(ctx, msg)

			// 消息处理完成后提交Offset
			if err := a.reader.CommitMessages(ctx, msg); err != nil {
				log.Printf("ERROR: failed to commit messages: %v", err)
			}
		}
	}()
}

// Stop 优雅地停止消费者。
func (a *OrderConsumerAdapter) Stop() {
	a.stopped = true
	a.reader.Close()
	a.wg.Wait()
	log.Printf("✅ Kafka Consumer Adapter stopped.")
}

// processMessage 反序列化消息并调用应用服务。
func (a *OrderConsumerAdapter) processMessage(parentCtx context.Context, msg kafka.Message) {
	// 解析消息体
	var event domain.OrderCreationRequested
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		log.Printf("ERROR: Failed to unmarshal event: %v. Message will be skipped.", err)
		// 在生产环境中，应将消息移至死信队列（DLQ）
		return
	}

	// 注意：这里的追踪上下文重建和超时控制逻辑也应该由适配器负责
	// 然后将最终的上下文传递给应用服务。
	// 为简化示例，我们直接调用应用服务的方法。

	propagator := otel.GetTextMapPropagator()
	headerCarrier := mq.KafkaHeaderCarrier(msg.Headers)
	ctx := propagator.Extract(parentCtx, &headerCarrier)

	// 调用应用服务来处理业务逻辑
	if err := a.appSvc.HandleOrderCreationEvent(ctx, &event); err != nil {
		log.Printf("ERROR: Failed to handle order creation event for order %s: %v", event.EventID, err)
		// 这里可以根据错误类型决定是否重试或发送到死信队列
	}
}
