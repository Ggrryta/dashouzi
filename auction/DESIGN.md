# 实时拍卖会系统 - 架构设计文档

## 1. 项目定位

实时拍卖会是整个训练计划的**收官综合项目**，融合了前面所有项目的技术能力：

| 来源项目 | 在拍卖会中的运用 |
|---|---|
| 秒杀 | 出价并发竞争 → Redis Lua 原子操作 |
| IM | 实时出价推送 → WebSocket 房间广播 |
| Feed | 出价历史流 → 时间线 + 游标分页 |
| 支付 | 成交结算 → 支付状态机 + 幂等 |
| 网关 | 限流 + 鉴权 → 中间件复用 |

**核心目标：** 在毫秒级实时推送 + 高并发出价竞争 + 严格状态机 + 资金安全的四重约束下，保证不出错。

## 2. 技术栈

| 层 | 选型 | 为什么 |
|---|---|---|
| 语言 | Go 1.26 | 和前序项目统一 |
| Web | Gin | HTTP API + WS 升级 |
| WebSocket | gorilla/websocket | 实时出价推送 |
| 数据库 | MySQL 8.0 | 拍卖会/出价记录/订单持久化 |
| 缓存 | Redis 7 | 当前最高价 + Lua 原子出价 + Pub/Sub 房间广播 |
| 消息队列 | Kafka | 出价日志异步落库 + 支付异步处理 |
| ORM | GORM | 仅用于拍卖会/订单持久化 |
| 容器 | Docker Compose | app + MySQL + Redis + Kafka |

> 为什么拍卖会既要 Kafka 又要 Redis Pub/Sub：Kafka 用于**持久化**出价日志（落库 + 回溯 + 对账），Redis Pub/Sub 用于**实时**广播出价更新（低延迟、不落盘）。职责分离，不可互相替代。

## 3. 整体架构

```
                         ┌──────────────────────┐
                         │     客户端 (浏览器)     │
                         │  HTTP API + WebSocket  │
                         └──────┬───────┬────────┘
                                │       │
                    HTTP REST   │       │  WS 长连接
                                ▼       ▼
                    ┌──────────────────────────────┐
                    │         Gin Server            │
                    │  ┌─────────┐  ┌────────────┐  │
                    │  │ REST API│  │ WS Hub     │  │
                    │  │ (出价等) │  │ (房间管理)  │  │
                    │  └────┬────┘  └─────┬──────┘  │
                    │       │             │         │
                    │  ┌────┴─────────────┴──────┐  │
                    │  │     中间件层              │  │
                    │  │  JWT 鉴权 + 限流 + 日志   │  │
                    │  └─────────────────────────┘  │
                    └──────┬──────────────┬─────────┘
                           │              │
              ┌────────────┘              └────────────┐
              ▼                                        ▼
    ┌─────────────────┐                    ┌───────────────────┐
    │     Redis 7      │                    │      MySQL 8       │
    │                  │                    │                    │
    │ • Lua 原子出价    │                    │ • 拍卖会表          │
    │ • 当前最高价      │                    │ • 出价记录表        │
    │ • 房间在线成员    │                    │ • 订单/支付表       │
    │ • Pub/Sub 广播   │                    │                    │
    └────────┬─────────┘                    └───────────────────┘
             │
             ▼
    ┌─────────────────┐
    │      Kafka       │
    │                  │
    │ • 出价日志异步落库 │
    │ • 支付异步处理    │
    │ • 拍卖到期事件    │
    └─────────────────┘
```

## 4. 核心流程

### 4.1 出价竞争（核心中的核心）

```
用户点击出价 ¥1200
    │
    ▼
┌──────────────┐
│ 1. 限流校验   │  ← 单用户 2 秒内只能出价 1 次（防恶意刷）
└──────┬───────┘
       │ 通过
       ▼
┌──────────────┐
│ 2. Redis Lua │  ← 原子操作：查当前价 → 比较 → 更新 → 记录出价人
│   原子出价    │     IF bid > current_price THEN update + record
└──────┬───────┘     ELSE return -1 (出价太低)
       │
  ┌────┴─────┐
  │          │
  ▼          ▼
出价成功    出价失败（低于当前价）
  │          │
  ▼          ▼
┌────────┐  ┌──────────┐
│3.Pub/  │  │ 返回失败  │
│Sub广播 │  └──────────┘
│通知全房 │
└──┬─────┘
   │
   ▼
┌──────────┐
│4.Kafka   │  ← 异步投递出价日志，Consumer 批量写 MySQL
│投递日志  │
└──────────┘
```

**为什么是这个流程：**
1. 限流在最外层挡住无效请求，避免穿透到 Redis
2. Redis Lua 原子出价，单次网络往返完成「查-比-更」，无并发竞争
3. 出价成功 ≠ 落库成功，Pub/Sub 实时广播 + Kafka 异步落库解耦
4. Consumer 批量写出价记录，offset 手提交保证不丢

### 4.2 Redis Lua 原子出价脚本

```lua
-- KEYS[1] = auction:current:{auction_id}  (当前最高价, String, 分为单位)
-- KEYS[2] = auction:bidder:{auction_id}   (当前最高出价人, String)
-- ARGV[1] = bid_amount   (出价金额，分为单位)
-- ARGV[2] = user_id      (出价人 ID)
-- ARGV[3] = min_increment (最小加价幅度，分为单位)

local current = tonumber(redis.call('get', KEYS[1]) or '0')
local bid = tonumber(ARGV[1])

-- 出价必须高于当前价 + 最小加价幅度
if bid < current + tonumber(ARGV[3]) then
    return -1  -- 出价过低
end

-- 原子更新最高价和出价人
redis.call('set', KEYS[1], ARGV[1])
redis.call('set', KEYS[2], ARGV[2])
redis.call('incr', 'auction:bidcount:' .. KEYS[1])  -- 出价次数统计

return 1  -- 出价成功
```

**为什么用 Lua 而不是先 GET 再 SET：**
- 两次 Redis 命令之间有并发窗口：A、B 同时出价 ¥1200，都看到当前价 ¥1000，都认为自己更高，竞态条件
- Lua 脚本在 Redis 单线程中执行，天然原子性

### 4.3 实时推送（WebSocket 房间广播）

```
出价成功后：

1. 出价服务 Publish 到 Redis 频道 auction:room:{auction_id}
   消息: {"type":"bid_update","auction_id":1,"price":1200,"bidder":"user_42","ts":...}

2. WS Hub 中每个 goroutine 订阅自己负责的拍卖间频道

3. Hub 收到消息 → 遍历该房间的所有在线连接 → 逐个推送

4. 客户端收到 → UI 更新当前价

         ┌──────────────────────────────────────┐
         │          Redis Pub/Sub               │
         │   auction:room:1  (频道)              │
         └──────┬──────────┬──────────┬─────────┘
                │          │          │
                ▼          ▼          ▼
            ┌──────┐  ┌──────┐  ┌──────┐
            │User A │  │User B │  │User C │
            │WS连接 │  │WS连接 │  │WS连接 │
            └──────┘  └──────┘  └──────┘
```

**为什么用 Pub/Sub 而不是 Hub 内存广播：**
- 单机内存广播只在一台机器内有效，多实例部署时 A 机器上的出价无法推给 B 机器上的连接
- Pub/Sub 让所有实例订阅同一频道，天然支持水平扩展

### 4.4 拍卖状态机

```
                    ┌──────────┐
        创建拍卖 ──→ │ pending  │  ← 等待开始（可编辑/取消）
                    └────┬─────┘
                         │ 到达 start_time
                         ▼
                    ┌──────────┐
                    │  live    │  ← 接受出价，实时推送
                    └────┬─────┘    倒计时进行中
                         │ 到达 end_time 或 手动落槌
                         ▼
                    ┌──────────┐
                    │ closing  │  ← 冻结出价，等待结算
                    └────┬─────┘
                    ╱         ╲
              有出价           无出价
                  ╱              ╲
        ┌──────────┐         ┌──────────┐
        │  sold    │         │  failed  │ ← 流拍
        └────┬─────┘         └──────────┘
             │ 买家支付成功
             ▼
        ┌──────────┐
        │ settled  │  ← 交易完成
        └──────────┘

状态流转规则（不可逆跳转）:
- pending → live:    到达开始时间，自动触发
- live → closing:    到达结束时间 或 拍卖师手动落槌
- closing → sold:    有有效出价
- closing → failed:  无出价
- sold → settled:    买家支付完成
- sold → failed:     超时未支付，重新拍卖
```

**状态机实现：** 复用博客系统的 DFA 思路，用合法转移表 `map[status]allowedNext{}` 强校验，非法跳转直接报错。

### 4.5 倒计时 & 自动落槌

采用 **Redis 过期键 + Keyspace Notification**（事件驱动，精确）：

```
拍卖进入 live 状态时:
  SET auction:timer:{id} 1 EX {duration_seconds}

Redis 配置:
  notify-keyspace-events Ex   (开启过期事件通知)

订阅:
  __keyevent@0__:expired

键过期时:
  → Consumer 收到 auction:timer:{id} 过期事件
  → 解析 auction_id
  → 触发落槌流程: live → closing → sold/failed
  → Pub/Sub 推送落槌通知到房间
```

**为什么选方案 B 而不是定时轮询：**
- 轮询有精度问题（1 秒扫一次，落槌最多延迟 1 秒）
- 过期键通知是事件驱动，精确到毫秒
- 轮询要扫描全表，拍卖多时开销大；过期键通知零额外查询

**兜底机制：** 过期事件可能因 Redis 重启丢失，因此额外有一个 1 分钟兜底扫描任务，检查所有 `live` 且已过 `end_time` 的拍卖，补做落槌。最终一致，不丢。

### 4.6 支付结算

```
拍卖成交后（status: sold）：

1. 生成订单（status: pending_payment）
2. 给买家推送 "成交待支付" 通知（WebSocket）
3. 买家 30 分钟内完成支付
4. 支付回调 → 幂等校验（order_id 去重）
5. 支付成功 → 状态 settled → 推送 "交易完成"

超时未支付:
  - 30 分钟后定时任务检查 pending_payment 订单
  - 订单 → cancelled
  - 拍卖 → failed（或重新拍卖给第二高出价人）
```

**支付简化说明：** 本项目不做真实支付通道接入，用 `POST /orders/:id/pay` 模拟支付回调，重点验证：订单状态机、支付幂等、超时取消。

## 5. 数据库设计

```sql
-- 拍卖会表
CREATE TABLE auctions (
    id           BIGINT AUTO_INCREMENT PRIMARY KEY,
    title        VARCHAR(256) NOT NULL,
    description  TEXT,
    item_image   VARCHAR(512),
    start_price  DECIMAL(10,2) NOT NULL,        -- 起拍价
    current_price DECIMAL(10,2) NOT NULL,       -- 当前最高价（落槌后为成交价）
    bid_increment DECIMAL(10,2) NOT NULL,       -- 最小加价幅度
    start_time   TIMESTAMP    NOT NULL,
    end_time     TIMESTAMP    NOT NULL,
    status       VARCHAR(16)  NOT NULL DEFAULT 'pending',
    -- pending/live/closing/sold/failed/settled
    winner_id    BIGINT,                        -- 中标者
    seller_id    BIGINT       NOT NULL,         -- 卖家
    created_at   TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_status_time (status, end_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 出价记录表
CREATE TABLE bids (
    id          BIGINT AUTO_INCREMENT PRIMARY KEY,
    auction_id  BIGINT       NOT NULL,
    user_id     BIGINT       NOT NULL,
    amount      DECIMAL(10,2) NOT NULL,
    created_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_auction_time (auction_id, created_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 订单/支付表
CREATE TABLE auction_orders (
    id              BIGINT AUTO_INCREMENT PRIMARY KEY,
    auction_id      BIGINT       NOT NULL,
    buyer_id        BIGINT       NOT NULL,
    amount          DECIMAL(10,2) NOT NULL,
    status          VARCHAR(16)  NOT NULL DEFAULT 'pending_payment',
    -- pending_payment/paid/cancelled/refunded
    paid_at         TIMESTAMP,
    expire_at       TIMESTAMP    NOT NULL,      -- 支付截止时间
    created_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_auction (auction_id)          -- 一个拍卖只有一个订单
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

## 6. Redis Key 设计

| Key | 类型 | 说明 |
|---|---|---|
| `auction:current:{id}` | String | 当前最高价（分为单位） |
| `auction:bidder:{id}` | String | 当前最高出价人 ID |
| `auction:bidcount:{id}` | String | 出价次数（统计用） |
| `auction:online:{id}` | Set | 房间在线成员 |
| `auction:timer:{id}` | String + TTL | 倒计时键，过期触发落槌 |
| `auction:ratelimit:{user_id}` | String + TTL | 出价限流计数器 |
| `auction:room:{id}` | Pub/Sub Channel | 房间实时消息广播 |

## 7. 接口设计

| 协议 | 路径 | 说明 |
|---|---|---|
| HTTP POST | `/api/v1/auctions` | 创建拍卖会（卖家） |
| HTTP GET | `/api/v1/auctions` | 拍卖列表（分页） |
| HTTP GET | `/api/v1/auctions/:id` | 拍卖详情 |
| HTTP POST | `/api/v1/auctions/:id/bid` | 出价（REST 备用，主走 WS） |
| HTTP GET | `/api/v1/auctions/:id/bids` | 出价历史（游标分页） |
| HTTP POST | `/api/v1/orders/:id/pay` | 支付（模拟回调） |
| WebSocket | `ws://host/ws/auction/:id?token={jwt}` | 加入拍卖间 |

WS 消息协议：

| 方向 | 消息 | 说明 |
|---|---|---|
| C→S | `{"type":"bid","amount":1200}` | 出价 |
| C→S | `{"type":"ping"}` | 心跳 |
| S→C | `{"type":"bid_update","price":1200,...}` | 出价更新推送 |
| S→C | `{"type":"hammer","auction_id":1,...}` | 落槌通知 |
| S→C | `{"type":"pong"}` | 心跳响应 |

## 8. 限流分层

```
用户请求 → 应用层限流(单用户 2s/次) → Redis Lua 原子出价 → Pub/Sub 广播 → Kafka 落库
```

出价限流用 Redis `INCR + EXPIRE`（不用 Go 内存），保证多实例共享计数。

## 9. 性能目标

| 指标 | 目标 |
|---|---|
| 出价 QPS | ≥ 5,000 |
| 出价 P99 延迟 | < 50ms |
| WS 推送延迟 | < 100ms |
| 并发出价不串价 | 最高价始终 = 实际最高出价 |
| 出价日志不丢 | Redis→Kafka→MySQL 一条不丢 |

## 10. 压测方案

```bash
# wrk 压测出价接口（REST 备用路径）
wrk -t8 -c200 -d30s --latency \
  -s scripts/bench/bid.lua \
  http://localhost:8080/api/v1/auctions/1/bid

# 验证不串价：并发出价后，Redis 最高价 = MySQL 最大出价
redis-cli GET auction:current:1
mysql -e "SELECT MAX(amount) FROM bids WHERE auction_id=1"

# 验证日志不丢：Kafka 投递数 = MySQL 出价记录数
```

WS 房间推送压测用自定义 Go 客户端，模拟 N 个连接同时订阅 + M 个出价，统计消息送达率。

## 11. Docker 编排

```yaml
services:
  app:        # Go 应用
  mysql:      # MySQL 8.0，拍卖会/出价/订单持久化
  redis:      # Redis 7，Lua 出价 + Pub/Sub + 倒计时
  zookeeper:  # Kafka 依赖
  kafka:      # 出价日志异步落库 + 支付异步
```

Redis 需配置 `notify-keyspace-events Ex` 以支持过期键通知。

## 12. 与前序项目的对比

| 维度 | 秒杀 | IM | 拍卖会 |
|---|---|---|---|
| 通信 | HTTP 短连接 | WebSocket 长连接 | **HTTP + WebSocket 混合** |
| 并发竞争 | 库存扣减 | 无 | **出价竞争（价高者得）** |
| 实时推送 | 无 | 点对点 | **房间广播（一对多）** |
| 状态机 | 简单 | 无 | **6 状态严格流转** |
| 资金安全 | 无 | 无 | **支付结算 + 超时退款** |
| 定时任务 | 无 | 心跳 | **倒计时落槌 + 支付超时** |

## 13. 目录结构

```
auction/
├── cmd/auction/main.go          # 入口
├── internal/
│   ├── auction/                  # 拍卖会管理（CRUD + 状态机）
│   ├── bid/                      # 出价核心（Lua 脚本 + 限流）
│   ├── room/                     # WebSocket 房间（Hub + 广播）
│   └── payment/                  # 支付结算（状态机 + 幂等）
├── pkg/
│   └── ws/                       # WebSocket 底层封装
├── configs/
├── deployments/
│   ├── docker-compose.yaml
│   └── migrations/
├── test/
│   ├── e2e/
│   └── bench/
└── api/
```
