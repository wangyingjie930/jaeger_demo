package port

import (
	"context"
	"nexus/internal/service/order/domain"
)

// NotificationProducer 是消息生产者的出站端口。
type NotificationProducer interface {
	// SendOrderCreated 发送订单创建成功的通知。
	SendOrderCreated(ctx context.Context, order *domain.Order) error

	// SendOrderCreationFailed 发送订单创建失败的通知。
	SendOrderCreationFailed(ctx context.Context, orderID, userID, reason string) error
}
