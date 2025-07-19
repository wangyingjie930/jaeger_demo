package adapter

import (
	"context"
	"net/url"
	"nexus/internal/pkg/constants"
	"nexus/internal/pkg/httpclient"
	"strconv"
)

// InventoryHTTPAdapter 实现了 port.InventoryService 接口。
type InventoryHTTPAdapter struct {
	client *httpclient.Client
}

// NewInventoryHTTPAdapter 创建一个新的库存服务适配器。
func NewInventoryHTTPAdapter(client *httpclient.Client) *InventoryHTTPAdapter {
	return &InventoryHTTPAdapter{client: client}
}

// ReserveStock 实现了预占库存的HTTP调用逻辑。
func (a *InventoryHTTPAdapter) ReserveStock(ctx context.Context, orderID string, items map[string]int) (map[string]int, error) {
	var rollbackBackItems = map[string]int{}
	// 在实际场景中，可能需要为每个item并发或批量调用
	for itemID, quantity := range items {
		params := url.Values{}
		params.Set("itemId", itemID)
		params.Set("quantity", strconv.Itoa(quantity))
		params.Set("orderId", orderID)
		if err := a.client.CallService(ctx, constants.InventoryService, constants.InventoryReservePath, params); err != nil {
			return rollbackBackItems, err // 一旦失败，立即返回
		}
		rollbackBackItems[itemID] = quantity
	}
	return nil, nil
}

// ReleaseStock 实现了释放库存的补偿逻辑。
func (a *InventoryHTTPAdapter) ReleaseStock(ctx context.Context, orderID string, items map[string]int) error {
	for itemID := range items {
		params := url.Values{}
		params.Set("itemId", itemID)
		params.Set("orderId", orderID)
		if err := a.client.CallService(ctx, constants.InventoryService, constants.InventoryReleasePath, params); err != nil {
			// 补偿操作失败是严重问题，需要记录错误，但通常不应中断其他补偿
			// 具体的错误处理策略可以在调用方决定
			return err
		}
	}
	return nil
}
