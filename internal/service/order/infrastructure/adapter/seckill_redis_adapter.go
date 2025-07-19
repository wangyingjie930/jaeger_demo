package adapter

import (
	"context"
	"fmt"
	"nexus/internal/pkg/redis"
	"nexus/internal/service/order/domain/port"
)

const (
	seckillScriptName       = "seckill"
	cancelSeckillScriptName = "cancel_seckill"
)

// SeckillRedisAdapter 是 port.SeckillService 接口的 Redis 实现。
type SeckillRedisAdapter struct {
	redisClient *redis.Client
}

// NewSeckillRedisAdapter 创建一个新的秒杀服务适配器实例。
// 它在创建时会加载所有需要的 Lua 脚本。
func NewSeckillRedisAdapter(redisClient *redis.Client) (*SeckillRedisAdapter, error) {
	// 在服务初始化时加载所需脚本
	if err := redisClient.LoadScriptFromContent(seckillScriptName, seckillScripts); err != nil {
		return nil, fmt.Errorf("failed to load critical seckill script: %w", err)
	}

	return &SeckillRedisAdapter{
		redisClient: redisClient,
	}, nil
}

// AttemptSeckill 实现了秒杀逻辑
func (a *SeckillRedisAdapter) AttemptSeckill(ctx context.Context, productID, userID string) (port.SeckillResult, error) {
	stockKey := fmt.Sprintf("seckill:stock:{%s}", productID)
	userSetKey := fmt.Sprintf("seckill:users:{%s}", productID)

	keys := []string{stockKey, userSetKey}
	args := []interface{}{userID}

	result, err := a.redisClient.RunScript(ctx, seckillScriptName, keys, args...)
	if err != nil {
		return 0, fmt.Errorf("seckill adapter failed to run script: %w", err)
	}

	code, ok := result.(int64)
	if !ok {
		return 0, fmt.Errorf("unexpected result type from Lua script: %T", result)
	}

	switch code {
	case 1:
		return port.SeckillResultSuccess, nil
	case 0:
		return port.SeckillResultSoldOut, nil
	case 2:
		return port.SeckillResultAlreadyPurchased, nil
	default:
		return 0, fmt.Errorf("unknown result code from seckill script: %d", code)
	}
}

// CancelSeckill 实现了秒杀的补偿逻辑
func (a *SeckillRedisAdapter) CancelSeckill(ctx context.Context, productID, userID string) error {
	return nil
}

// PrepareSeckillProduct (测试和管理用) 初始化秒杀商品库存
func (s *SeckillRedisAdapter) PrepareSeckillProduct(ctx context.Context, productID string, stock int) error {
	stockKey := fmt.Sprintf("seckill:stock:{%s}", productID)
	userSetKey := fmt.Sprintf("seckill:users:{%s}", productID)

	// 使用 pipeline 提高效率
	pipe := s.redisClient.GetClient().Pipeline()
	pipe.Set(ctx, stockKey, stock, 0)
	pipe.Del(ctx, userSetKey)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to prepare seckill product: %w", err)
	}
	return nil
}

var seckillScripts = `
-- scripts/seckill.lua

-- KEYS[1]: 秒杀库存的 Key, 例如: seckill:stock:{product_123}
-- KEYS[2]: 秒杀成功用户列表的 Key, 例如: seckill:users:{product_123}
-- ARGV[1]: 当前尝试下单的用户 ID, 例如: user-vip-789

-- 1. 检查用户是否已购买过
if redis.call('sismember', KEYS[2], ARGV[1]) == 1 then
    return 2 -- 返回 2, 代表用户重复购买
end

-- 2. 获取当前库存
local stock = tonumber(redis.call('get', KEYS[1]))

-- 3. 检查库存是否充足
if stock and stock > 0 then
    -- 4. 库存充足，减库存
    redis.call('decr', KEYS[1])
    -- 5. 将用户ID添加到成功列表
    redis.call('sadd', KEYS[2], ARGV[1])
    return 1 -- 返回 1, 代表秒杀成功
else
    -- 6. 库存不足
    return 0 -- 返回 0, 代表已售罄
end
`
