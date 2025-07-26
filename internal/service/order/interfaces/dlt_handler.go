// internal/service/order/interfaces/dlt_handler.go
package interfaces

import (
	"context"
	"github.com/wangyingjie930/nexus-pkg/logger"
	"github.com/wangyingjie930/nexus-pkg/mq"
	"sync"

	"github.com/segmentio/kafka-go"
)

// DltConsumerAdapter 监听死信队列并记录日志
type DltConsumerAdapter struct {
	reader  *kafka.Reader
	wg      sync.WaitGroup
	stopped bool
}

func NewDltConsumerAdapter(reader *kafka.Reader) *DltConsumerAdapter {
	return &DltConsumerAdapter{
		reader: reader,
	}
}

func (a *DltConsumerAdapter) Start(ctx context.Context) error {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		logger.Ctx(ctx).Info().Str("topic", a.reader.Config().Topic).Msg("✅ DLT Consumer Adapter started.")
		for {
			if a.stopped {
				return
			}
			msg, err := a.reader.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					logger.Ctx(ctx).Info().Msg("🛑 DLT Consumer Adapter shutting down.")
					return
				}
				continue
			}

			// 记录死信消息详情
			logDeadLetter(ctx, msg)

			// DLT中的消息总是直接提交，因为它们已经被“处理”了（即记录日志）
			a.reader.CommitMessages(ctx, msg)
		}
	}()
	return nil
}

func (a *DltConsumerAdapter) Stop(ctx context.Context) {
	a.stopped = true
	a.reader.Close()
	a.wg.Wait()
	logger.Ctx(ctx).Info().Str("topic", a.reader.Config().Topic).Msg("✅ DLT Consumer Adapter stopped.")
}

func logDeadLetter(ctx context.Context, msg kafka.Message) {
	headers := make(map[string]string)
	for _, h := range msg.Headers {
		headers[h.Key] = string(h.Value)
	}

	// 使用结构化日志记录，便于后续分析
	logger.Ctx(ctx).Error().
		Str("reason", "dead_letter_message_received").
		Str("original_topic", headers[mq.HeaderOriginalTopic]).
		Str("original_partition", headers[mq.HeaderOriginalPartition]).
		Str("original_offset", headers[mq.HeaderOriginalOffset]).
		Str("exception_fqcn", headers[mq.HeaderExceptionFqcn]).
		Str("exception_message", headers[mq.HeaderExceptionMessage]).
		Str("key", string(msg.Key)).
		Str("value", string(msg.Value)).
		Msg("🚨 CRITICAL: Dead letter message received")
}
