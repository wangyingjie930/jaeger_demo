package port

import (
	"context"
)

// InventoryService 是库存服务的出站端口。
type InventoryService interface {
	// ReserveStock 为给定的订单预占库存。
	ReserveStock(ctx context.Context, orderID string, items map[string]int) (rollbackItems map[string]int, err error)

	// ReleaseStock 是 ReserveStock 的补偿操作，用于释放预占的库存。
	ReleaseStock(ctx context.Context, orderID string, items map[string]int) error
}
