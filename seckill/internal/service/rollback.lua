-- 回滚扣减：投递 Kafka 失败时回滚 Redis 库存与已购集合
-- KEYS[1]: seckill:stock:{item_id}
-- KEYS[2]: seckill:bought:{item_id}
-- ARGV[1]: user_id

redis.call('incr', KEYS[1])
redis.call('srem', KEYS[2], ARGV[1])
return 1
