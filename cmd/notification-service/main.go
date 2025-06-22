// cmd/notification-service/main.go
package main

import (
	"context"
	"encoding/json"
	"errors"
	"jaeger-demo/internal/mq" // 新增
	"jaeger-demo/internal/tracing"
	"log"
	"time"

	"go.opentelemetry.io/otel/codes"

	"github.com/segmentio/kafka-go" // 新增
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace" // 新增
)

const (
	serviceName    = "notification-service"
	jaegerEndpoint = "http://localhost:14268/api/traces"
)

var (
	tracer            = otel.Tracer(serviceName)
	kafkaBrokers      = []string{"localhost:9092"}
	notificationTopic = "notifications"
	consumerGroupID   = "notification-group"
)

// NotificationEvent 定义了从 Kafka 接收的消息结构
type NotificationEvent struct {
	UserID  string `json:"userID"`
	Message string `json:"message"`
}

func main() {
	tp, err := tracing.InitTracerProvider(serviceName, jaegerEndpoint)
	if err != nil {
		log.Fatalf("failed to initialize tracer provider: %v", err)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}()

	// 不再启动 HTTP 服务器
	// http.HandleFunc("/send_notification", sendNotificationHandler)
	// log.Println("Notification Service listening on :8083")
	// log.Fatal(http.ListenAndServe(":8083", nil))

	// 启动 Kafka 消费者
	reader := mq.NewKafkaReader(kafkaBrokers, notificationTopic, consumerGroupID)
	defer reader.Close()

	log.Println("Notification Service started as a Kafka consumer for topic:", notificationTopic)

	// 循环消费消息
	for {
		// 使用 context.Background()，因为这是一个后台任务的根
		msg, err := reader.ReadMessage(context.Background())
		if err != nil {
			log.Printf("could not read message: %v", err)
			continue
		}

		// 处理消息
		go processNotification(msg)
	}
}

// processNotification 处理从 Kafka 收到的单条消息
func processNotification(msg kafka.Message) {
	// 1. 从消息头中提取追踪上下文
	// 注意：这里的 ctx 是基于提取出的上下文创建的，而不是继承自某个现有请求
	ctx := mq.ExtractTraceContext(context.Background(), msg.Headers)

	// 添加调试日志来检查追踪上下文
	if spanCtx := trace.SpanContextFromContext(ctx); spanCtx.IsValid() {
		log.Printf("Extracted valid trace context: traceID=%s, spanID=%s",
			spanCtx.TraceID().String(), spanCtx.SpanID().String())
	} else {
		log.Printf("Warning: No valid trace context found in message headers")
		// 如果没有有效的追踪上下文，创建一个新的根 span
		ctx, _ = tracer.Start(context.Background(), "notification-service.ProcessNotification.Orphaned")
	}

	// 2. 基于提取的上下文创建新的 Span
	// 这会将当前操作链接到上游（order-service）的追踪链路中
	spanOpts := []trace.SpanStartOption{
		trace.WithAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.destination", msg.Topic),
			attribute.Int("messaging.kafka.partition", msg.Partition),
			attribute.Int64("messaging.kafka.message.offset", msg.Offset),
			attribute.String("messaging.kafka.message.key", string(msg.Key)),
		),
		// 将其设置为消费者类型的 Span
		trace.WithSpanKind(trace.SpanKindConsumer),
	}
	ctx, span := tracer.Start(ctx, "notification-service.ProcessNotification", spanOpts...)
	defer span.End()

	var event NotificationEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		log.Printf("failed to unmarshal message: %v", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return
	}

	span.SetAttributes(attribute.String("user.id", event.UserID))
	if event.UserID == "0" {
		// 模拟发送通知失败
		log.Printf("Failed to send notification to user %s: %s", event.UserID, event.Message)
		err := errors.New("invalid user id")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return
	}

	// 模拟发送通知的耗时
	log.Printf("Sending notification to user %s: %s", event.UserID, event.Message)
	time.Sleep(50 * time.Millisecond) // 模拟业务逻辑
	span.AddEvent("Notification sent successfully")

	log.Printf("Successfully processed notification for user %s", event.UserID)
}
