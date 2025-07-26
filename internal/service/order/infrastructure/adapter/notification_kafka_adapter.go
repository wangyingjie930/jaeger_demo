package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/segmentio/kafka-go"
	"github.com/wangyingjie930/nexus-pkg/mq"
	"nexus/internal/service/order/domain"
	"time"
)

const (
	notificationTopic = "notifications"
	paymentTimeout    = 3 * time.Second // 这个配置可以从外部注入
)

// NotificationKafkaAdapter 实现了 port.NotificationProducer 接口。
type NotificationKafkaAdapter struct {
	writer *kafka.Writer
}

// NewNotificationKafkaAdapter 创建一个新的通知生产者适配器。
func NewNotificationKafkaAdapter(writer *kafka.Writer) *NotificationKafkaAdapter {
	return &NotificationKafkaAdapter{writer: writer}
}

// SendOrderCreated 实现了发送订单创建成功通知的逻辑。
func (a *NotificationKafkaAdapter) SendOrderCreated(ctx context.Context, order *domain.Order) error {
	message := fmt.Sprintf(
		"Your order %s is waiting for payment. Please complete it within %v.",
		order.ID, paymentTimeout,
	)
	if order.PromoID != "" {
		message = fmt.Sprintf("Your VIP promotion order (%s) has been successfully created!", order.PromoID)
	}

	event := domain.NotificationEvent{ // 使用领域事件结构
		UserID:      order.UserID,
		Message:     message,
		PromotionID: order.PromoID,
	}
	eventBytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal notification event: %w", err)
	}

	// 调用通用的 mq.ProduceMessage，它会自动处理追踪上下文注入
	return mq.ProduceMessage(ctx, a.writer, []byte(order.UserID), eventBytes)
}

// SendOrderCreationFailed 实现了发送订单创建失败通知的逻辑。
func (a *NotificationKafkaAdapter) SendOrderCreationFailed(ctx context.Context, orderID, userID, reason string) error {
	// ... 在这里实现发送失败通知的逻辑 ...
	return nil
}

// Close 关闭底层的Kafka writer。
func (a *NotificationKafkaAdapter) Close() error {
	return a.writer.Close()
}
