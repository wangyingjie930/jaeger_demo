package port

import (
	"context"
	"errors"
)

var (
	ErrSeckillSoldOut          = errors.New("product is sold out")
	ErrSeckillAlreadyPurchased = errors.New("user has already purchased this product")
)

// SeckillResult 是秒杀结果的枚举
type SeckillResult int

const (
	SeckillResultSuccess SeckillResult = iota + 1
	SeckillResultSoldOut
	SeckillResultAlreadyPurchased
)

// SeckillService 是秒杀服务的出站端口。
// 它定义了秒杀相关的业务能力，由基础设施层实现。
type SeckillService interface {
	// AttemptSeckill 尝试执行秒杀操作。
	AttemptSeckill(ctx context.Context, productID, userID string) (SeckillResult, error)

	// CancelSeckill 是 AttemptSeckill 的补偿操作。
	CancelSeckill(ctx context.Context, productID, userID string) error
}
