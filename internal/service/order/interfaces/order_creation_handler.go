// internal/service/order/infrastructure/adapter/kafka_consumer_adapter.go
package interfaces

import (
	"context"
	"encoding/json"
	"nexus/internal/pkg/logger"
	"nexus/internal/pkg/mq"
	"nexus/internal/service/order/application"
	"nexus/internal/service/order/domain"
	"sync"
	"time"

	"go.opentelemetry.io/otel"

	"github.com/segmentio/kafka-go"
)

// OrderConsumerAdapter 是一个驱动适配器，它监听Kafka消息并驱动应用服务。
type OrderConsumerAdapter struct {
	reader  *kafka.Reader
	appSvc  *application.OrderApplicationService // <-- 依赖应用服务层的接口
	wg      sync.WaitGroup
	stopped bool

	failureHandler *mq.FailureHandler // ✨ 注入FailureHandler
	delay          time.Duration      // ✨ 新增延迟字段
}

// NewOrderConsumerAdapter 创建一个新的Kafka消费者适配器。
func NewOrderConsumerAdapter(reader *kafka.Reader, appSvc *application.OrderApplicationService, failureHandler *mq.FailureHandler) *OrderConsumerAdapter {
	return &OrderConsumerAdapter{
		reader:         reader,
		appSvc:         appSvc,
		failureHandler: failureHandler,
	}
}

func (a *OrderConsumerAdapter) SetDelay(d time.Duration) {
	a.delay = d
}

// Start 开始监听Kafka主题。这是一个长期运行的方法。
func (a *OrderConsumerAdapter) Start(ctx context.Context) error {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		logger.Ctx(ctx).Printf("✅ Kafka Consumer Adapter started for topic '%s'.", a.reader.Config().Topic)
		for {
			if a.stopped {
				return
			}
			// 我们使用FetchMessage而不是ReadMessage，以便更好地控制退出逻辑
			msg, err := a.reader.FetchMessage(ctx)
			if err != nil {
				// 如果是上下文取消导致的错误，则正常退出
				if ctx.Err() != nil {
					logger.Ctx(ctx).Error().Err(ctx.Err()).Msg("🛑 Kafka Consumer Adapter shutting down.")
					return
				}
				logger.Ctx(ctx).Printf("ERROR: could not read message: %v. Retrying...", err)
				time.Sleep(1 * time.Second) // 避免快速失败循环
				continue
			}

			// ✨ 延迟处理逻辑
			if a.delay > 0 {
				deliveryTime := msg.Time.Add(a.delay)
				logger.Ctx(ctx).Info().Any("checkTime", time.Now().Before(deliveryTime)).
					Any("deliveryTime", deliveryTime.Format(time.DateTime)).
					Any("now", time.Now().Format(time.DateTime)).
					Msgf("检查消息!")

				time.Sleep(deliveryTime.Sub(time.Now()))
			}

			propagator := otel.GetTextMapPropagator()
			headerCarrier := mq.KafkaHeaderCarrier(msg.Headers)
			newCtx := propagator.Extract(ctx, &headerCarrier)

			// ✨ 将处理逻辑移入一个带错误返回的函数
			processingErr := a.processMessage(newCtx, msg)

			if processingErr != nil {
				// ✨ 如果处理失败，调用FailureHandler
				a.failureHandler.Handle(newCtx, msg, processingErr)
			}

			// ✨ 无论成功或失败（已移交），都提交Offset
			if err := a.reader.CommitMessages(ctx, msg); err != nil {
				logger.Ctx(ctx).Error().Err(err).Msg("Failed to commit messages")
			}
		}
	}()
	return nil
}

// Stop 优雅地停止消费者。
func (a *OrderConsumerAdapter) Stop(ctx context.Context) {
	a.stopped = true
	a.reader.Close()
	a.wg.Wait()
	logger.Ctx(ctx).Printf("✅ Kafka Consumer Adapter stopped.")
}

// processMessage 反序列化消息并调用应用服务。
func (a *OrderConsumerAdapter) processMessage(ctx context.Context, msg kafka.Message) error {
	//return errors.New("connection timeout")
	// 解析消息体
	var event domain.OrderCreationRequested
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return err
	}

	// 调用应用服务来处理业务逻辑
	return a.appSvc.HandleOrderCreationEvent(ctx, &event)
}
