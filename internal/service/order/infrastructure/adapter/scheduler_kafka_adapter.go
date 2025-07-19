package adapter

import (
	"context"
	"encoding/json"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel/trace"
	"nexus/internal/pkg/mq"
	"nexus/internal/service/order/domain"
	"time"
)

const (
	// 定义延迟主题和真实的目标主题
	delayTopic      = "delay_topic_5s" // 假设我们使用5秒延迟
	realTopic       = "order-timeout-check-topic"
	paymentDeadline = 5 * time.Second // 与延迟主题匹配
)

// SchedulerKafkaAdapter 实现了 port.DelayScheduler 接口。
type SchedulerKafkaAdapter struct {
	delayWriter *kafka.Writer
}

// NewSchedulerKafkaAdapter 创建一个新的延迟任务调度器适配器。
func NewSchedulerKafkaAdapter(brokers []string) *SchedulerKafkaAdapter {
	return &SchedulerKafkaAdapter{
		delayWriter: mq.NewKafkaWriter(brokers, delayTopic),
	}
}

// SchedulePaymentTimeout 实现了发送延迟消息的逻辑。
func (a *SchedulerKafkaAdapter) SchedulePaymentTimeout(ctx context.Context, orderID, userID string, items []string, creationTime time.Time) error {
	taskEvent := domain.OrderTimeoutCheckEvent{
		TraceID:      trace.SpanFromContext(ctx).SpanContext().TraceID().String(),
		OrderID:      orderID,
		UserID:       userID,
		Items:        items,
		CreationTime: time.Now(),
	}
	taskBytes, _ := json.Marshal(taskEvent)

	delayTimestamp := creationTime.Add(5 * time.Second).Format(time.RFC3339)

	msg := kafka.Message{
		Key:   []byte(orderID),
		Value: taskBytes,
		Headers: []kafka.Header{
			{Key: "real-topic", Value: []byte(realTopic)},
			{Key: "delay-timestamp", Value: []byte(delayTimestamp)},
		},
	}
	mq.InjectTraceContext(ctx, &msg.Headers)

	return a.delayWriter.WriteMessages(ctx, msg)
}

// Close 关闭底层的Kafka writer。
func (a *SchedulerKafkaAdapter) Close() error {
	return a.delayWriter.Close()
}
