// internal/service/order/domain/repository.go
package domain

import "context"

// OrderRepository 定义了订单聚合的持久化接口。
// 它位于领域层，但由基础设施层实现。
type OrderRepository interface {
	// Save 保存一个订单聚合（用于创建或更新）。
	Save(ctx context.Context, order *Order) error

	// FindByID 根据 ID 查找一个订单聚合。
	FindByID(ctx context.Context, id string) (*Order, error)

	// UpdateState 是一个更具体的更新操作，可能为了性能优化而存在。
	UpdateState(ctx context.Context, id string, state State) error
}
