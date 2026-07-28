-- 秒杀 Lua 脚本：原子扣库存 + 一人一单
-- KEYS[1]: seckill:stock:{item_id}
-- KEYS[2]: seckill:bought:{item_id}
-- ARGV[1]: user_id

local stock_key = KEYS[1]
local bought_key = KEYS[2]
local user_id = ARGV[1]

-- 1. 检查是否已购买
if redis.call('sismember', bought_key, user_id) == 1 then
    return -1  -- 已购买
end

-- 2. 检查库存
local stock = tonumber(redis.call('get', stock_key) or '0')
if stock <= 0 then
    return 0   -- 库存不足
end

-- 3. 扣库存 + 记录用户
redis.call('decr', stock_key)
redis.call('sadd', bought_key, user_id)
return 1       -- 抢购成功
