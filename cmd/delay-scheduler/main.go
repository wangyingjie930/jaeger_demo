package main

import (
	"context"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"jaeger-demo/internal/pkg/mq"
	"jaeger-demo/internal/pkg/tracing"

	"github.com/ouqiang/timewheel"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

const (
	serviceName = "delay-scheduler"
)

// 定义延迟Topic的前缀和支持的延迟级别
const (
	delayTopicPrefix = "delay_topic_"
)

var (
	jaegerEndpoint = getEnv("JAEGER_ENDPOINT", "http://localhost:14268/api/traces")
	kafkaBrokers   = strings.Split(getEnv("KAFKA_BROKERS", "localhost:9092"), ",")
	// 动态获取所有需要订阅的延迟主题
	delayTopics = getDelayTopics()
	tracer      = otel.Tracer(serviceName)
)

// Task 定义了时间轮中存储的任务
type Task struct {
	RealTopic   string
	MessageKey  []byte
	MessageBody []byte
	Headers     []kafka.Header
}

func main() {
	// 1. 初始化追踪
	tp, err := tracing.InitTracerProvider(serviceName, jaegerEndpoint)
	if err != nil {
		log.Fatalf("failed to initialize tracer provider: %v", err)
	}
	defer tp.Shutdown(context.Background())

	// 2. 初始化一个时间轮
	// 参数：时间间隔，轮盘的刻度数，执行任务的回调函数
	tw := timewheel.New(1*time.Second, 3600, func(data interface{}) {
		task, ok := data.(Task)
		if !ok {
			log.Printf("ERROR: Invalid task type in timewheel")
			return
		}
		// 任务到期，投递到真实Topic
		publishToRealTopic(task)
	})
	tw.Start() // 启动时间轮
	defer tw.Stop()

	log.Printf("✅ Delay Scheduler started. TimeWheel is running...")

	// 3. 为每个延迟Topic启动一个消费者Goroutine
	var wg sync.WaitGroup
	for _, topic := range delayTopics {
		wg.Add(1)
		go func(t string) {
			defer wg.Done()
			consumeAndSchedule(t, tw)
		}(topic)
	}
	wg.Wait()
}

// consumeAndSchedule 是每个消费者的核心逻辑
func consumeAndSchedule(topic string, tw *timewheel.TimeWheel) {
	reader := mq.NewKafkaReader(kafkaBrokers, topic, serviceName+"-group")
	defer reader.Close()
	log.Printf("Listening for messages on delay topic: %s", topic)

	for {
		msg, err := reader.ReadMessage(context.Background())
		if err != nil {
			log.Printf("ERROR: could not read message from %s: %v", topic, err)
			continue
		}

		ctx := mq.ExtractTraceContext(context.Background(), msg.Headers)
		_, span := tracer.Start(ctx, "scheduler.ScheduleMessage")

		log.Printf("read message from %s:%v", topic, msg)

		// 从Header中获取真实Topic和精确的延迟时间戳
		realTopic := getHeader(msg.Headers, "real-topic")
		delayTimestampStr := getHeader(msg.Headers, "delay-timestamp")

		if realTopic == "" || delayTimestampStr == "" {
			log.Printf("ERROR: 'real-topic' or 'delay-timestamp' header is missing. Dropping message from topic %s.", topic)
			span.SetStatus(codes.Error, "Missing required headers")
			span.End()
			continue
		}

		delayTimestamp, err := time.Parse(time.RFC3339, delayTimestampStr)
		if err != nil {
			log.Printf("ERROR: Invalid 'delay-timestamp' format: %v", err)
			span.SetStatus(codes.Error, "Invalid timestamp format")
			span.End()
			continue
		}

		delay := time.Until(delayTimestamp)
		if delay <= 0 {
			// 消息已经到期或无需延迟，立即投递
			publishToRealTopic(Task{
				RealTopic:   realTopic,
				MessageKey:  msg.Key,
				MessageBody: msg.Value,
				Headers:     msg.Headers,
			})
			span.AddEvent("Message already expired, publishing immediately.")
		} else {
			// 将任务添加到时间轮
			tw.AddTimer(delay, "delay-task", Task{
				RealTopic:   realTopic,
				MessageKey:  msg.Key,
				MessageBody: msg.Value,
				Headers:     msg.Headers,
			})
			log.Printf("Scheduled message for topic %s to be delivered in %v", realTopic, delay)
			span.SetAttributes(
				attribute.String("real_topic", realTopic),
				attribute.Int64("delay_ms", delay.Milliseconds()),
			)
		}
		span.End()
	}
}

// publishToRealTopic 将任务消息发布到最终的业务Topic
func publishToRealTopic(task Task) {
	ctx := mq.ExtractTraceContext(context.Background(), task.Headers)
	ctx, span := tracer.Start(ctx, "scheduler.PublishToRealTopic")
	defer span.End()

	span.SetAttributes(
		attribute.String("messaging.destination", task.RealTopic),
	)

	writer := mq.NewKafkaWriter(kafkaBrokers, task.RealTopic)
	defer writer.Close()

	// 关键：将原始消息的所有信息（包括追踪头）原封不动地发送出去
	err := mq.ProduceMessage(ctx, writer, task.MessageKey, task.MessageBody)
	if err != nil {
		log.Printf("ERROR: Failed to publish message to real topic %s: %v", task.RealTopic, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to publish")
	} else {
		log.Printf("SUCCESS: Message published to real topic %s", task.RealTopic)
	}
}

// --- 辅助函数 ---

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getDelayTopics() []string {
	return []string{
		delayTopicPrefix + "5s",
		delayTopicPrefix + "1m",
		delayTopicPrefix + "10m",
	}
}

func getHeader(headers []kafka.Header, key string) string {
	for _, h := range headers {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}
