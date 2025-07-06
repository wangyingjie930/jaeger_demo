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