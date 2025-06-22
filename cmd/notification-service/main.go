// cmd/notification-service/main.go
package main

import (
	"context"
	"encoding/json"
	"errors"
	"jaeger-demo/internal/mq"
	"jaeger-demo/internal/tracing"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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

// <<<<<<< 改造点: 更新事件结构 >>>>>>>>>
type NotificationEvent struct {
	UserID      string `json:"userID"`
	Message     string `json:"message"`
	PromotionID string `json:"promotion_id,omitempty"`
}

func main() {
	tp, err := tracing.InitTracerProvider(serviceName, jaegerEndpoint)
	if err != nil {
		log.Fatalf("failed to initialize tracer provider: %v", err)
	}
	defer tp.Shutdown(context.Background())

	reader := mq.NewKafkaReader(kafkaBrokers, notificationTopic, consumerGroupID)
	defer reader.Close()

	log.Println("Notification Service started as a Kafka consumer for topic:", notificationTopic)

	for {
		msg, err := reader.ReadMessage(context.Background())
		if err != nil {
			log.Printf("could not read message: %v", err)
			continue
		}
		go processNotification(msg)
	}
}

func processNotification(msg kafka.Message) {
	ctx := mq.ExtractTraceContext(context.Background(), msg.Headers)

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
		log.Printf("failed to unmarshal message: %v", err)
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
	log.Printf("Sending notification to user %s: %s", event.UserID, event.Message)
	if event.PromotionID != "" {
		log.Printf("--> This is a special promotion notification: %s", event.PromotionID)
		span.AddEvent("Personalized promotion notification sent")
	} else {
		span.AddEvent("Standard notification sent")
	}

	time.Sleep(50 * time.Millisecond)
	log.Printf("Successfully processed notification for user %s", event.UserID)
}
