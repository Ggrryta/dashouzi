# 商品秒杀系统 - 架构设计文档

## 1. 项目定位

博客系统是 Level 1「单体夯实」，秒杀系统是 Level 2「高并发入门」。

**核心目标：** 从「功能正确」升级到「高并发下功能正确」——不超卖、扛得住、可扩展。

## 2. 技术栈

| 层 | 选型 | 为什么 |
|---|---|---|
| 语言 | Go 1.23 | 高性能、大厂主流 |
| Web | Gin | 和博客统一技术栈 |
| 数据库 | MySQL 8.0 | 订单持久化，国内电商主流 |
| 缓存 | Redis 7 | 库存预热、Lua 原子扣减 |
| ORM | GORM | 仅用于订单写入 |
| 消息队列 | Kafka | 高吞吐削峰 + 消息持久化 + 回溯重放 |
| 容器 | Docker Compose | app + MySQL + Redis + Kafka + Zookeeper |

> 为什么选 Kafka 而不是 RabbitMQ：秒杀需要百万级 QPS 削峰 + 消息不丢持久化 + 对账时可回溯重放消息。RabbitMQ 更适合复杂路由场景，秒杀不需要。

## 3. 秒杀核心流程

```
用户点击抢购
    │
    ▼
┌─────────────┐
│ 1. 限流校验  │  ← 令牌桶，单用户 N 秒内只能请求一次
└──────┬──────┘
       │ 通过
       ▼
┌─────────────┐
│ 2. Redis     │  ← Lua 脚本原子操作：查库存 + 扣库存 + 记录用户
│ 库存扣减     │     IF stock > 0 AND user not in set THEN decr + sadd
└──────┬──────┘
       │
  ┌────┴────┐
  │         │
  ▼         ▼
扣减成功   库存不足/已抢过
  │         │
  ▼         ▼
┌────────┐  ┌──────────┐
│3.Kafka │  │ 返回失败  │
│Producer│  └──────────┘
│投递消息 │
└──┬─────┘
   │
   ▼
┌──────────┐
│4.Consumer│  ← 拉取 Kafka 消息，批量写入 MySQL
│ 批量写   │     offset 管理 + 可多实例扩展
│ MySQL    │
└──────────┘
```

**为什么是这个流程：**
1. 限流在最外层，挡住无效请求，避免穿透到 Redis
2. Redis Lua 原子扣库存，单次网络往返完成「查-判-扣」，无并发竞争
3. 扣减成功 ≠ 下单成功，Kafka 异步投递消息，削峰
4. Consumer 批量写 MySQL，offset 手提交保证不丢，Consumer Group 支持水平扩展

## 4. 关键设计决策

### 4.1 库存预热

秒杀开始前，将 MySQL 中的库存加载到 Redis：

```go
// 预热
redis.Set("seckill:stock:1", 1000)
```

秒杀期间所有库存操作只读 Redis，不走 DB。

### 4.2 Lua 原子扣减

```lua
-- 单次网络往返，原子执行
local stock_key = KEYS[1]       -- seckill:stock:1
local bought_key = KEYS[2]      -- seckill:bought:1
local user_id = ARGV[1]

-- 1. 检查是否已购买
if redis.call('sismember', bought_key, user_id) == 1 then
    return -1  -- 已购买
end

-- 2. 检查库存并扣减
local stock = tonumber(redis.call('get', stock_key) or '0')
if stock <= 0 then
    return 0   -- 库存不足
end

redis.call('decr', stock_key)
redis.call('sadd', bought_key, user_id)
return 1         -- 扣减成功
```

**为什么用 Lua 而不是先 GET 再 SET：**
- 两次 Redis 命令之间有并发窗口：A 查到库存 1，B 也查到库存 1，两个都扣成 0，超卖
- Lua 脚本在 Redis 单线程中执行，天然原子性

### 4.3 返回值的含义

| 返回值 | 含义 | HTTP 响应 |
|---|---|---|
| 1 | 抢购成功 | 200 |
| -1 | 已抢过（一人一单） | 403 |
| 0 | 库存不足 | 400 |

### 4.4 异步下单（削峰）

```go
// Producer：扣减成功后，投递到 Kafka
producer.Send(&kafka.Message{
    Topic: "seckill.orders",
    Key:   fmt.Sprintf("%d", userID),     // 同一用户进同一分区，保证顺序
    Value: []byte(fmt.Sprintf(`{"user_id":%d,"item_id":%d}`, userID, itemID)),
})

// Consumer：独立 goroutine 拉取并批量写 MySQL
for {
    msg, err := consumer.ReadMessage(ctx)
    createOrderInDB(msg.Value)
    consumer.CommitMessages(ctx, msg)  // 手动提交 offset，保证不丢
}
```

**为什么用 Kafka 而不是同步写 DB：**
- 秒杀峰值可能是平时的 1000 倍，同步写 DB 直接打挂
- Kafka 顺序写磁盘 + 零拷贝，吞吐量百万级 QPS
- ISR 机制保证消息不丢
- 手动提交 offset：订单写 MySQL 成功后才 commit，crash 后可重试
- Topic 可以有多个 Consumer Group，后续接数据统计/风控无需改代码

### 4.5 缓存与 DB 最终一致性

秒杀结束后，定时对账：

```go
// 对账脚本
redisStock := redis.Get("seckill:stock:1")
dbStock := db.Query("SELECT stock FROM seckill_items WHERE id = 1")
// 不一致时以 DB 为准，修正 Redis
```

### 4.6 限购一人一单

用 Redis Set 记录已购买用户，Lua 脚本中 `sismember` 校验。

## 5. 数据库设计

```sql
-- 秒杀场次表
CREATE TABLE seckill_sessions (
    id         BIGINT AUTO_INCREMENT PRIMARY KEY,
    name       VARCHAR(128) NOT NULL,
    start_time TIMESTAMP    NOT NULL,
    end_time   TIMESTAMP    NOT NULL,
    status     VARCHAR(16)  NOT NULL DEFAULT 'pending', -- pending/active/finished
    created_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 秒杀商品表
CREATE TABLE seckill_items (
    id           BIGINT AUTO_INCREMENT PRIMARY KEY,
    session_id   BIGINT       NOT NULL,
    title        VARCHAR(256) NOT NULL,
    price        DECIMAL(10,2) NOT NULL,       -- 秒杀价
    origin_price DECIMAL(10,2) NOT NULL,       -- 原价
    total_stock  INT          NOT NULL,          -- 总库存（预热到 Redis 的初始值）
    sold_count   INT          NOT NULL DEFAULT 0, -- 已售数量（对账用）
    image_url    VARCHAR(512),
    created_at   TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (session_id) REFERENCES seckill_sessions(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 秒杀订单表
CREATE TABLE seckill_orders (
    id         BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id    BIGINT       NOT NULL,
    session_id BIGINT       NOT NULL,
    item_id    BIGINT       NOT NULL,
    price      DECIMAL(10,2) NOT NULL,
    status     VARCHAR(16)  NOT NULL DEFAULT 'paid', -- paid/cancelled
    created_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_user_item (user_id, item_id)  -- 一人一单兜底
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

## 6. Redis Key 设计

| Key | 类型 | 说明 |
|---|---|---|
| `seckill:stock:{item_id}` | String | 当前剩余库存 |
| `seckill:bought:{item_id}` | Set | 已购买用户 ID 集合 |
| `seckill:ratelimit:{user_id}` | String | 用户限流计数器 |

## 7. 接口设计

| 方法 | 路径 | 说明 | 限流 |
|---|---|---|---|
| GET | `/api/v1/seckill/items` | 秒杀商品列表 | - |
| GET | `/api/v1/seckill/items/:id` | 秒杀商品详情 | - |
| POST | `/api/v1/seckill/buy` | 执行秒杀 | 1次/N秒/用户 |
| GET | `/api/v1/seckill/result/:item_id` | 查询抢购结果 | - |

## 8. 高并发优化要点

### 8.1 读优化
- 商品列表和详情加 Redis 缓存，TTL 1 分钟
- 静态页面 CDN（本次不做）

### 8.2 写优化
- 库存只在 Redis 操作，秒杀结束后批量回写 DB
- 下单异步，MQ 消费者控制 DB 写入速率

### 8.3 限流分层

```
用户请求 → 网关限流(本次不做) → 应用层令牌桶(本次实现) → Redis Lua → MQ
```

## 9. 压测目标

| 指标 | 目标 |
|---|---|
| QPS | ≥ 10,000 |
| P99 延迟 | < 100ms |
| 不超卖 | 实际售出 ≤ 设定库存 |
| 一人一单 | 同一用户同一商品只能购买一次 |

## 10. 压测方案

```bash
# wrk 压测
wrk -t12 -c400 -d30s --latency \
  -s seckill.lua \
  http://localhost:8080/api/v1/seckill/buy

# 验证不超卖
redis-cli GET seckill:stock:1
# 应等于初始库存 - 实际订单数

# 验证一人一单
redis-cli SCARD seckill:bought:1
# 应等于实际订单数
```

## 11. Docker 编排

```yaml
services:
  app:        # Go 应用
  mysql:      # MySQL 8.0，订单持久化
  redis:      # Redis 7，库存缓存 + 限流计数器
  zookeeper:  # Kafka 依赖，集群协调
  kafka:      # 消息削峰，异步下单
```

Kafka 依赖 ZooKeeper（Kraft 模式 KRaft 在 Kafka 3.3+ 可去掉 ZK，但 Docker 镜像仍以 ZK 模式最成熟）。

## 12. 与博客系统的对比

| 维度 | 博客系统 | 秒杀系统 |
|---|---|---|
| 架构 | 单体，同步 | 单体 + Redis + 异步队列 |
| 瓶颈 | 无 | 库存扣减的并发竞争 |
| 核心难点 | 状态机、认证 | 原子性、削峰、一致性 |
| 新引入 | DFA、RBAC | Lua 脚本、MQ 思想、压测 |
| 性能要求 | 无 | QPS 10,000+ |
