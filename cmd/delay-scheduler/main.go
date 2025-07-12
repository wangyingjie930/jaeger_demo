// cmd/delay-scheduler-polling/main.go
package main

import (
	"context"
	"jaeger-demo/internal/pkg/mq"
	"jaeger-demo/internal/pkg/tracing"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	serviceName = "delay-scheduler-polling"
)

// 定义支持的延迟级别和对应的主题
var delayLevels = map[string]time.Duration{
	"delay_topic_5s":  5 * time.Second,
	"delay_topic_1m":  1 * time.Minute,
	"delay_topic_10m": 10 * time.Minute,
}

var (
	jaegerEndpoint = getEnv("JAEGER_ENDPOINT", "http://localhost:14268/api/traces")
	kafkaBrokers   = strings.Split(getEnv("KAFKA_BROKERS", "localhost:9092"), ",")
	tracer         = otel.Tracer(serviceName)
)

// Scheduler 负责管理和运行轮询任务
type Scheduler struct {
	level       string        // 延迟级别名称, e.g., "delay_topic_5s"
	delay       time.Duration // 对应的延迟时长, e.g., 5s
	kafkaReader *kafka.Reader
	// 为每个级别维护一个独立的 writer, 避免并发问题
	kafkaWriters map[string]*kafka.Writer // key: realTopic, value: writer
	writerLock   sync.Mutex
}

// NewScheduler 创建一个针对特定延迟级别的新调度器
func NewScheduler(level string, delay time.Duration) *Scheduler {
	reader := mq.NewKafkaReader(kafkaBrokers, level, serviceName+"-group-"+level)
	return &Scheduler{
		level:        level,
		delay:        delay,
		kafkaReader:  reader,
		kafkaWriters: make(map[string]*kafka.Writer),
	}
}

// StartPolling 启动定时轮询器
func (s *Scheduler) StartPolling(ctx context.Context, interval time.Duration) {
	log.Printf("✅ Polling scheduler for level '%s' started, checking every %v", s.level, interval)
	// 每个延迟等级一个独立的 ticker
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	defer s.kafkaReader.Close()
	defer s.closeWriters()

	for {
		select {
		case <-ticker.C:
			s.checkAndPublish(ctx)
		case <-ctx.Done():
			log.Printf("🛑 Shutting down polling for level '%s'", s.level)
			return
		}
	}
}

// checkAndPublish 是轮询的核心逻辑
func (s *Scheduler) checkAndPublish(parentCtx context.Context) {
	for {
		// 1. 使用 FetchMessage 而不是 ReadMessage, 这样我们可以控制提交流程
		// FetchMessage 不会自动提交 offset
		msg, err := s.kafkaReader.FetchMessage(parentCtx)
		if err != nil {
			if err == context.Canceled || err.Error() == "context deadline exceeded" {
				// 正常退出或超时，不是错误
			} else {
				// 如果没有消息可读，kafka-go 会返回一个错误，我们在这里退出循环，等待下一次 tick
				// log.Printf("DEBUG: No new messages in '%s', waiting for next tick. Err: %v", s.level, err)
			}
			break // 退出 for 循环
		}

		propagator := otel.GetTextMapPropagator()
		header := mq.KafkaHeaderCarrier(msg.Headers)
		spanCtx := propagator.Extract(parentCtx, &header)
		now := time.Now().UTC()
		ctx, span := tracer.Start(spanCtx, "scheduler.CheckAndPublish", trace.WithAttributes(
			attribute.String("delay.level", s.level),
			attribute.String("now", now.Format(time.DateTime)),
			attribute.String("msg.Time", msg.Time.Format(time.DateTime)),
			attribute.String("delay", msg.Time.Add(s.delay).Format(time.DateTime)),
		))

		// 2. 计算理论投递时间 (消息存储时间 + 延迟)
		// Kafka 消息的 Time 字段记录了其进入主题的时间戳
		deliveryTime := msg.Time.Add(s.delay)

		// 3. 判断是否到期
		if now.After(deliveryTime) {
			// 消息到期，进行投递
			log.Printf("INFO: Message in '%s' is due. DeliveryTime: %v, Now: %v. Publishing...", s.level, deliveryTime, time.Now())

			realTopic := s.getHeader(msg.Headers, "real-topic")
			if realTopic == "" {
				log.Printf("ERROR: 'real-topic' header missing in message from '%s'. Skipping.", s.level)
				// 这种错误消息也需要提交，否则会一直被重复消费
				if err := s.kafkaReader.CommitMessages(ctx, msg); err != nil {
					log.Printf("ERROR: Failed to commit message after skipping: %v", err)
				}
				span.End()
				continue // 处理下一条消息
			}

			// 投递到真实 Topic
			if err := s.publish(ctx, realTopic, msg); err != nil {
				log.Printf("ERROR: Failed to publish message to real topic '%s': %v", realTopic, err)
				// 投递失败，不能提交 offset，等待下次轮询重试
				span.RecordError(err)
				span.SetStatus(codes.Error, "Failed to publish to real topic")
				span.End()
				break // 退出循环，防止处理后续消息
			}

			// 投递成功，提交 offset
			if err := s.kafkaReader.CommitMessages(ctx, msg); err != nil {
				log.Printf("ERROR: Failed to commit message for '%s' after successful publish: %v", s.level, err)
				span.RecordError(err)
				span.End()
				// 即使提交失败，也退出循环，避免消息重复投递。Kafka consumer group 会处理好 offset
				break
			}
			log.Printf("SUCCESS: Message from '%s' published to '%s' and committed.", s.level, realTopic)
			span.AddEvent("MessagePublishedAndCommitted", trace.WithAttributes(attribute.String("real.topic", realTopic)))
			span.End()
		} else {
			// 队头消息未到期，无需再检查后续消息
			// log.Printf("DEBUG: Head message in '%s' not yet due (DeliveryTime: %v). Waiting for next tick.", s.level, deliveryTime)
			span.AddEvent("HeadMessageNotDue")
			span.End()
			break // 退出 for 循环
		}
	}
}

// publish 将消息投递到真实业务主题
func (s *Scheduler) publish(ctx context.Context, realTopic string, msg kafka.Message) error {
	s.writerLock.Lock()
	writer, exists := s.kafkaWriters[realTopic]
	if !exists {
		writer = mq.NewKafkaWriter(kafkaBrokers, realTopic)
		s.kafkaWriters[realTopic] = writer
	}
	s.writerLock.Unlock()

	// 重新构造消息，并注入追踪上下文
	publishMsg := kafka.Message{
		Key:   msg.Key,
		Value: msg.Value,
	}
	traceCtx := mq.ExtractTraceContext(ctx, msg.Headers)
	mq.InjectTraceContext(traceCtx, &publishMsg.Headers)

	return writer.WriteMessages(ctx, publishMsg)
}

// closeWriters 安全地关闭所有 writer
func (s *Scheduler) closeWriters() {
	s.writerLock.Lock()
	defer s.writerLock.Unlock()
	for topic, writer := range s.kafkaWriters {
		if err := writer.Close(); err != nil {
			log.Printf("ERROR: Failed to close writer for topic %s: %v", topic, err)
		}
	}
}

func (s *Scheduler) getHeader(headers []kafka.Header, key string) string {
	for _, h := range headers {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}

func main() {
	tp, err := tracing.InitTracerProvider(serviceName, jaegerEndpoint)
	if err != nil {
		log.Fatalf("failed to initialize tracer provider: %v", err)
	}
	defer tp.Shutdown(context.Background())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	// 为每个延迟级别启动一个独立的调度器 goroutine
	for level, delay := range delayLevels {
		wg.Add(1)
		scheduler := NewScheduler(level, delay)
		go func() {
			defer wg.Done()
			// 轮询周期为1秒，与您描述的一致
			scheduler.StartPolling(ctx, 1*time.Second)
		}()
	}

	log.Println("All polling schedulers are running.")
	wg.Wait()
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
