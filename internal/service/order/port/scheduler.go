package port

import (
	"context"
	"time"
)

// DelayScheduler 是延迟任务调度器的出站端口。
type DelayScheduler interface {
	// SchedulePaymentTimeout 安排一个在未来执行的订单支付超时检查任务。
	SchedulePaymentTimeout(ctx context.Context, orderID, userID string, items []string, creationTime time.Time) error
}
