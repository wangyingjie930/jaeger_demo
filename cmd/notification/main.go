// cmd/notification-service/main.go
package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/wangyingjie930/nexus-pkg/logger"
	"github.com/wangyingjie930/nexus-pkg/mq"
	"github.com/wangyingjie930/nexus-pkg/tracing"
	"os"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// getEnv 从环境变量中读取配置。
// 如果环境变量不存在，则返回提供的默认值。
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

const (
	serviceName       = "notification-service"
	notificationTopic = "notifications"
	consumerGroupID   = "notification-group"
)

var (
	jaegerEndpoint = getEnv("JAEGER_ENDPOINT", "http://localhost:14268/api/traces")
	// 读取逗号分隔的 broker 列表
	kafkaBrokers = strings.Split(getEnv("KAFKA_BROKERS", "localhost:9092"), ",")

	tracer = otel.Tracer(serviceName)
)

// <<<<<<< 改造点: 更新事件结构 >>>>>>>>>
type NotificationEvent struct {
	UserID      string `json:"userId"`
	Message     string `json:"message"`
	PromotionID string `json:"promotion_id,omitempty"`
}

func main() {
	logger.Init(serviceName)

	tp, err := tracing.InitTracerProvider(serviceName, jaegerEndpoint)
	if err != nil {
		logger.Logger.Fatal().Err(err).Msg("failed to initialize tracer provider")
	}
	defer tp.Shutdown(context.Background())

	reader := mq.NewKafkaReader(kafkaBrokers, notificationTopic, consumerGroupID)
	defer reader.Close()

	logger.Logger.Println("Notification Service started as a Kafka consumer for topic:", notificationTopic)

	for {
		ctx := context.Background()
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			logger.Ctx(ctx).Error().Err(err).Msg("could not read message")
			continue
		}
		ctx = mq.ExtractTraceContext(ctx, msg.Headers)
		go processNotification(ctx, msg)
	}
}

func processNotification(ctx context.Context, msg kafka.Message) {
	spanOpts := []trace.SpanStartOption{
		trace.WithAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.destination", msg.Topic),
			attribute.String("messaging.kafka.message.key", string(msg.Key)),
		),
		trace.WithSpanKind(trace.SpanKindConsumer),
	}
	ctx, span := tracer.Start(ctx, "notification-service.ProcessNotification", spanOpts...)
	defer span.End()

	var event NotificationEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		logger.Ctx(ctx).Error().Err(err).Msg("failed to unmarshal message")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return
	}

	// <<<<<<< 改造点: 记录更丰富的属性 >>>>>>>>>
	span.SetAttributes(
		attribute.String("user.id", event.UserID),
		attribute.String("promotion.id", event.PromotionID),
	)

	if event.UserID == "0" {
		err := errors.New("invalid user id")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return
	}

	// 模拟发送通知的耗时
	logger.Ctx(ctx).Printf("Sending notification to user %s: %s", event.UserID, event.Message)
	if event.PromotionID != "" {
		logger.Ctx(ctx).Printf("--> This is a special promotion notification: %s", event.PromotionID)
		span.AddEvent("Personalized promotion notification sent")
	} else {
		span.AddEvent("Standard notification sent")
	}

	time.Sleep(50 * time.Millisecond)
	logger.Ctx(ctx).Printf("Successfully processed notification for user %s", event.UserID)
}
