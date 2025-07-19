package port

import (
	"context"
	"nexus/internal/service/order/domain"
)

// FraudDetectionService 是欺诈检测服务的出站端口。
// 应用层通过此接口与外部欺诈检测系统交互。
type FraudDetectionService interface {
	// CheckFraud 对给定的订单信息执行欺诈检查。
	// 它封装了所有与外部服务通信的技术细节。
	CheckFraud(ctx context.Context, order *domain.Order) error
}
