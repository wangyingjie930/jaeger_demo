package order

import (
	"context"
	"fmt"
	"nexus/internal/pkg/redis"
)

// SeckillResultCode 是秒杀脚本返回的业务状态码
type SeckillResultCode int

const (
	ResultSoldOut          SeckillResultCode = 0 // 已售罄
	ResultSuccess          SeckillResultCode = 1 // 成功
	ResultAlreadyPurchased SeckillResultCode = 2 // 重复购买
)

// SeckillService 封装了秒杀的业务逻辑
type SeckillService struct {
	redisClient *redis.Client
	scriptName  string
}

// NewSeckillService 创建一个新的秒杀服务实例
func NewSeckillService(redisClient *redis.Client) *SeckillService {
	// 脚本的加载和管理由业务方决定
	// 这里假设脚本在项目根目录的 scripts/seckill.lua
	const scriptName = "seckill"
	const scriptPath = "scripts/seckill.lua"

	// 在服务初始化时加载所需脚本
	if err := redisClient.LoadScriptFromFile(scriptName, scriptPath); err != nil {
		// 如果脚本加载失败，这是一个严重问题，程序应该 panic
		panic(fmt.Sprintf("Failed to load critical seckill script: %v", err))
	}

	return &SeckillService{
		redisClient: redisClient,
		scriptName:  scriptName,
	}
}

// AttemptSeckill 尝试执行秒杀操作
func (s *SeckillService) AttemptSeckill(ctx context.Context, productID, userID string) (SeckillResultCode, error) {
	// 1. 构建业务相关的 Key
	stockKey := fmt.Sprintf("seckill:stock:{%s}", productID)
	userSetKey := fmt.Sprintf("seckill:users:{%s}", productID)

	keys := []string{stockKey, userSetKey}
	args := []interface{}{userID}

	// 2. 调用通用的 RunScript 方法
	result, err := s.redisClient.RunScript(ctx, s.scriptName, keys, args...)
	if err != nil {
		return -1, fmt.Errorf("seckill service failed to run script: %w", err)
	}

	// 3. 解析和翻译结果
	// 从 Redis 返回的 interface{} 转换为业务需要的 int64
	code, ok := result.(int64)
	if !ok {
		return -1, fmt.Errorf("unexpected result type from Lua script: %T", result)
	}

	// 4. 返回具有明确业务含义的状态码
	return SeckillResultCode(code), nil
}

// PrepareSeckillProduct (测试和管理用) 初始化秒杀商品库存
func (s *SeckillService) PrepareSeckillProduct(ctx context.Context, productID string, stock int) error {
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
