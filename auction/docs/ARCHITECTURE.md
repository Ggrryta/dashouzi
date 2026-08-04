# 实时拍卖系统 - 架构设计文档

> 本文档描述"怎么做"。需求来源见 `docs/REQUIREMENTS.md`，接口契约见 `api/openapi.yaml`。

---

## 1. 项目定位

实时拍卖系统是整个训练计划的**收官综合项目**，融合前序项目能力并升级为**多房间、每房间多商品**的实时拍卖场景：

| 来源项目 | 在本系统中的运用 |
|---|---|
| 秒杀 | 出价并发竞争 → Redis Lua 原子操作 + 限流 |
| IM | 实时推送 → WebSocket 房间广播 + 心跳管理 |
| Feed | 出价历史 → 时间线 + 游标分页 |
| 网关 | 限流 + 鉴权 → 中间件复用 |

**核心目标：** 在多房间、多商品、长连接、高并发的约束下，保证出价不串价、落槌不遗漏、推送低延迟、断线可重连。

---

## 2. 技术栈选型

| 层 | 选型 | 原因 |
|---|---|---|
| 语言 | Go 1.25.0 | 当前稳定版，与前序项目兼容 |
| Web 框架 | Gin | HTTP API + WebSocket 升级 |
| WebSocket | gorilla/websocket | 成熟、与 Gin 配合简单 |
| 数据库 | MySQL 8.0 | 房间、商品、出价记录持久化 |
| 缓存 | Redis 7 | Lua 原子出价、当前最高价、Pub/Sub 房间广播、倒计时、限流 |
| 容器 | Docker Compose | app + MySQL + Redis |
| ORM | GORM | 房间/商品/出价记录持久化 |

> 为什么本期不用 Kafka：
> - PRD 已明确本期不做支付结算
> - 出价历史可直接由出价服务同步写 MySQL，足够支撑本期性能目标
> - Kafka 异步落库可作为二期增强项，在落槌后对账和审计场景引入

---

## 3. 整体架构

```
                         ┌──────────────────────────┐
                         │       客户端              │
                         │  HTTP API + WebSocket    │
                         └──────┬─────────┬─────────┘
                                │         │
                    HTTP REST   │         │  WS 长连接
                                ▼         ▼
                    ┌──────────────────────────────┐
                    │         Gin Server            │
                    │  ┌─────────┐  ┌────────────┐  │
                    │  │ REST API│  │ WS Hub     │  │
                    │  │(房间/商品│  │(房间管理+  │  │
                    │  │/出价历史)│  │  广播)     │  │
                    │  └────┬────┘  └─────┬──────┘  │
                    │       │             │         │
                    │  ┌────┴─────────────┴──────┐  │
                    │  │     中间件层             │  │
                    │  │  X-User-Id + 限流 + 日志 │  │
                    │  └─────────────────────────┘  │
                    └──────┬──────────────┬─────────┘
                           │              │
              ┌────────────┘              └────────────┐
              ▼                                        ▼
    ┌─────────────────┐                    ┌───────────────────┐
    │     Redis 7      │                    │      MySQL 8       │
    │                  │                    │                    │
    │ • Lua 原子出价    │                    │ • 房间表           │
    │ • 当前最高价      │                    │ • 商品表           │
    │ • 房间在线成员    │                    │ • 出价记录表        │
    │ • Pub/Sub 广播   │                    │                    │
    │ • 倒计时 TTL     │                    │                    │
    │ • 限流计数器     │                    │                    │
    └─────────────────┘                    └───────────────────┘
```

---

## 4. 核心流程

### 4.1 出价竞争流程

系统提供两条出价路径：
- **WebSocket 主路径**：观众在房间长连接上发送 `bid` 消息，实时性强
- **REST 压测/备用路径**：`POST /api/v1/items/{itemId}/bids`，用于性能验收和前端降级

两条路径共享同一套出价核心逻辑。

```
用户提交出价（HTTP 或 WS）
              │
              ▼
    ┌───────────────────┐
    │ 0. 输入校验        │  ← amount > 0、itemId 合法、用户身份有效
    └─────────┬─────────┘
              │ 通过
              ▼
    ┌───────────────────┐
    │ 1. 商品状态校验    │  ← 商品必须是 live 状态
    └─────────┬─────────┘
              │ 通过
              ▼
    ┌───────────────────┐
    │ 2. 限流校验        │  ← 单用户对单商品 2 秒内只能出价 1 次（原子 SET NX EX）
    └─────────┬─────────┘
              │ 通过
              ▼
    ┌───────────────────┐
    │ 3. Redis Lua      │  ← 原子操作：查当前价 → 比较 → 更新最高价/出价人/出价次数
    │   原子出价         │     IF amount >= next_min_bid THEN update
    └─────────┬─────────┘     ELSE return -1
              │
       ┌──────┴──────┐
       │             │
       ▼             ▼
    出价成功        出价失败
       │             │
       ▼             ▼
┌─────────────┐  ┌─────────────┐
│ 4. 同步写    │  │ 返回 error  │
│   MySQL出价  │  │  消息        │
│   记录       │  └─────────────┘
└──────┬──────┘
       │
       ▼
┌───────────────────┐
│ 5. MySQL 写成功？  │
└───────┬───────┬───┘
        │       │
        │ 是    │ 否
        ▼       ▼
   ┌────────┐ ┌─────────────────┐
   │ 6. 广播 │ │ 标记异常 + 补偿  │
   │ bid_   │ │ 记录日志，后台任务│
   │ update │ │ 尝试重新写库      │
   └────────┘ └─────────────────┘
```

**持久化与一致性边界：**
- Redis Lua 成功即代表该出价在竞争中被接受，Redis 中的最高价变为事实
- 随后必须同步写 MySQL 出价记录
- 若 MySQL 写入失败，向客户端返回"系统繁忙"，但 Redis 已更新；后台补偿任务会扫描 Redis 中有但 MySQL 中缺失的出价记录，重新写入
- 正常路径下，广播只在 MySQL 写成功后发生，保证"用户看到广播即代表记录已落库"

### 4.2 Redis Lua 原子出价脚本

```lua
-- KEYS[1] = auction:item:{item_id}:current_price  (当前最高价, String, 分为单位)
-- KEYS[2] = auction:item:{item_id}:top_bidder     (当前最高出价人, String)
-- KEYS[3] = auction:item:{item_id}:bid_count      (有效出价次数, String)
-- ARGV[1] = amount       (出价金额，分为单位)
-- ARGV[2] = bidder_id    (出价人 ID)
-- ARGV[3] = start_price  (起拍价，分为单位)
-- ARGV[4] = min_increment(最小加价幅度，分为单位)

local current = tonumber(redis.call('get', KEYS[1]) or '0')
local amount = tonumber(ARGV[1])
local start_price = tonumber(ARGV[3])
local min_increment = tonumber(ARGV[4])

-- 计算下一个合法出价下限
local min_bid = 0
if current == 0 then
    min_bid = start_price
else
    min_bid = current + min_increment
end

-- 出价必须满足最低要求
if amount < min_bid then
    return -1  -- 出价过低
end

-- 原子更新最高价、出价人和出价次数
redis.call('set', KEYS[1], amount)
redis.call('set', KEYS[2], ARGV[2])
redis.call('incr', KEYS[3])

return 1  -- 出价成功
```

**为什么用 Lua：**
- 先 GET 再 SET 存在竞态窗口：两个客户端同时读到相同当前价，都认为自己的出价合法
- Lua 在 Redis 单线程执行，保证「读-比-写」三步原子性
- 所有出价竞争最终串行化到 Redis，是系统不串价的根基

**Redis 数据重建：**
- 商品从 `pending` 进入 `live` 时，从 MySQL 查询该商品当前最大出价记录
- 若有记录，恢复 `current_price` 和 `top_bidder`；若无，初始化为 0
- 落槌后保留 Redis 数据一段时间（如 1 小时）供重连查询，之后清理

### 4.3 实时推送流程

```
出价成功或状态变更后：

1. 服务端 Publish 到 Redis 频道 auction:room:{room_id}
   消息示例:
   {"type":"bid_update","itemId":1,"bidderId":42,"amount":120000,"currentPrice":120000,"nextMinBid":121000}
   {"type":"item_state","itemId":1,"status":"live","currentPrice":100000,"nextMinBid":100000}

2. 每个 Server 实例启动时订阅所有房间频道：
   - 每个实例维护一个全局 Redis Pub/Sub 连接
   - 使用模式订阅 `psubscribe auction:room:*`，避免动态管理单个频道
   - 收到消息后按 room_id 路由到本地 Hub

3. Hub 收到消息 → 找到该房间在本机的所有在线连接 → 逐个异步推送
   - 推送使用非阻塞 goroutine 池，避免单用户网络慢阻塞全房间
   - 向每个连接写入时设置超时，超时的连接标记为待清理

4. 客户端收到 → UI 更新对应商品的价格或状态
```

**为什么用 Pub/Sub 模式订阅：**
- 单机内存广播无法跨实例工作
- 全量 `psubscribe auction:room:*` 让每个实例都能收到所有房间事件
- 频道按房间隔离，消息中携带 itemId，避免消息串播
- 房间数增长时不需要动态增删订阅

### 4.4 商品状态机

```
                    ┌──────────┐
        创建商品 ──→ │ pending  │  ← 待开始，卖家可删除
                    └────┬─────┘
                         │ 到达 start_time
                         ▼
                    ┌──────────┐
                    │   live   │  ← 接受出价，实时推送
                    └────┬─────┘
                         │ 到达 end_time
                         ▼
                    ┌──────────┐
                    │ closing  │  ← 冻结出价，执行落槌
                    └────┬─────┘
                    ╱         ╲
              有出价           无出价
                  ╱              ╲
        ┌──────────┐         ┌──────────┐
        │  sold    │         │  failed  │ ← 流拍
        └──────────┘         └──────────┘
```

状态流转规则（不可逆）：
- `pending → live`：到达开始时间，自动触发，广播 `item_state`
- `live → closing`：到达结束时间，自动触发，广播 `item_state`
- `closing → sold`：存在有效出价，广播 `hammer`
- `closing → failed`：无有效出价，广播 `hammer`

**实现方式：** 用合法转移表 `map[status]allowedNext{}` 强校验，非法跳转直接报错。

### 4.5 倒计时与自动落槌

采用 **Redis 过期键 + 高频兜底扫描 + 数据库调度** 的三层保险机制：

```
商品进入 live 状态时:
  SET auction:item:{item_id}:timer 1 EX {duration_seconds}

Redis 配置:
  notify-keyspace-events Ex

订阅:
  __keyevent@0__:expired

键过期时:
  → 解析 item_id
  → 将商品状态 live → closing
  → 检查是否有有效出价（Redis bid_count > 0 或 current_price > 0）
  → closing → sold 或 failed
  → 更新 MySQL 商品状态、winner_id、current_price、bid_count
  → 清理 Redis 热数据（保留 1 小时后过期）
  → Pub/Sub 向房间广播 hammer 事件
```

**兜底扫描：**
- 启动一个 5 秒周期的扫描任务
- 查询 MySQL 中所有 `status = 'live'` 且 `end_time <= NOW()` 的商品
- 对每个商品执行落槌流程
- 保证 Redis 过期事件丢失时，落槌延迟不超过 5 秒

**为什么选过期键通知 + 兜底扫描：**
- 过期键通知是事件驱动，精度高
- 兜底扫描保证最终一致，不遗漏
- 5 秒周期在性能与及时性之间取得平衡

### 4.6 断线重连与单连接约束

```
用户连接建立:
  → /ws/rooms/{roomId}?userId=42
  → 在 Redis Hash 中记录 userId → "conn_id@instance_id"
  → 若该 userId 已存在旧映射:
      - 通过 Pub/Sub 向旧连接所在实例发送 "kick:userId:roomId" 消息
      - 旧实例收到后强制关闭旧连接
  → 新连接加入本地 Hub 和 Redis 在线集合

用户断线:
  → 连接关闭，Hub 从本地在线集合移除
  → 删除 Redis Hash 中该 userId 的映射（仅当映射指向当前实例时）
  → 从 Redis Set 中移除 userId

用户重连:
  → 重新握手，新连接替换旧连接
  → 发送 { "type": "join" }
  → 服务端从 Redis 读取该房间所有商品的当前状态
  → 组装 room_state 消息推送给该用户
```

**Redis 连接映射设计：**
- `auction:room:{room_id}:online`：Set，存储在线 userId
- `auction:room:{room_id}:conn`：Hash，field = userId，value = "conn_id@instance_id"

**状态恢复来源：**
- 商品价格、最高出价人从 Redis 读取（毫秒级）
- 商品状态从 MySQL 读取（防止 Redis 与 DB 不一致）
- 历史出价不同步，用户主动查询 `/api/v1/items/{itemId}/bids`

**重连风暴保护：**
- 对重连请求做令牌桶限流，单用户 1 秒内最多重连 3 次
- room_state 快照结果本地缓存 500ms，同一房间内多个用户同时重连时共享缓存
- Redis 读取使用 Pipeline 批量获取多个商品状态

---

## 5. 数据库设计

```sql
-- 房间表
CREATE TABLE rooms (
    id          BIGINT AUTO_INCREMENT PRIMARY KEY,
    name        VARCHAR(128) NOT NULL,
    description VARCHAR(2048),
    owner_id    BIGINT       NOT NULL COMMENT '房间创建者',
    status      VARCHAR(16)  NOT NULL DEFAULT 'online' COMMENT 'online/closed',
    created_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_owner (owner_id),
    INDEX idx_status_created (status, created_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 商品表
CREATE TABLE items (
    id            BIGINT AUTO_INCREMENT PRIMARY KEY,
    room_id       BIGINT       NOT NULL COMMENT '所属房间',
    seller_id     BIGINT       NOT NULL COMMENT '卖家',
    title         VARCHAR(128) NOT NULL,
    description   VARCHAR(2048),
    image_url     VARCHAR(512),
    start_price   BIGINT       NOT NULL COMMENT '起拍价，单位分',
    min_increment BIGINT       NOT NULL COMMENT '最小加价幅度，单位分',
    status        VARCHAR(16)  NOT NULL DEFAULT 'pending' COMMENT 'pending/live/closing/sold/failed',
    start_time    TIMESTAMP    NOT NULL,
    end_time      TIMESTAMP    NOT NULL,
    current_price BIGINT       NOT NULL DEFAULT 0 COMMENT '当前最高价，0 表示无人出价',
    bid_count     INT          NOT NULL DEFAULT 0 COMMENT '有效出价次数',
    winner_id     BIGINT       NULL COMMENT '中标者',
    created_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_room_status (room_id, status),
    INDEX idx_status_end_time (status, end_time),
    INDEX idx_start_time (start_time),
    FOREIGN KEY (room_id) REFERENCES rooms(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 出价记录表
CREATE TABLE bids (
    id         BIGINT AUTO_INCREMENT PRIMARY KEY,
    item_id    BIGINT       NOT NULL,
    bidder_id  BIGINT       NOT NULL,
    amount     BIGINT       NOT NULL COMMENT '出价金额，单位分',
    bid_time   TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '出价时间，毫秒精度',
    created_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_item_time (item_id, bid_time DESC),
    INDEX idx_item_bidder (item_id, bidder_id),
    FOREIGN KEY (item_id) REFERENCES items(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

---

## 6. Redis Key 设计

| Key | 类型 | 说明 |
|---|---|---|
| `auction:item:{item_id}:current_price` | String | 当前最高价（分为单位），0 表示无人出价 |
| `auction:item:{item_id}:top_bidder` | String | 当前最高出价人 ID |
| `auction:item:{item_id}:bid_count` | String | 有效出价次数 |
| `auction:item:{item_id}:timer` | String + TTL | 倒计时键，过期触发落槌；落槌后保留 1 小时 |
| `auction:item:{item_id}:ratelimit:{user_id}` | String + TTL | 单用户对单商品出价限流计数器 |
| `auction:room:{room_id}:online` | Set | 房间内在线用户 ID 集合 |
| `auction:room:{room_id}:conn` | Hash | userId → conn_id@instance_id，用于跨实例踢旧连接 |
| `auction:room:{room_id}` | Pub/Sub Channel | 房间实时消息广播频道 |

---

## 7. 接口概览

详细字段见 `api/openapi.yaml`。

| 协议 | 路径 | 说明 |
|---|---|---|
| HTTP POST | `/api/v1/rooms` | 创建房间 |
| HTTP GET | `/api/v1/rooms` | 房间列表（游标分页） |
| HTTP GET | `/api/v1/rooms/{roomId}` | 房间详情 |
| HTTP POST | `/api/v1/rooms/{roomId}/items` | 创建商品 |
| HTTP GET | `/api/v1/rooms/{roomId}/items` | 房间内商品列表（支持 status 筛选） |
| HTTP DELETE | `/api/v1/items/{itemId}` | 删除商品（仅 pending 状态） |
| HTTP GET | `/api/v1/items/{itemId}` | 商品详情 |
| HTTP POST | `/api/v1/items/{itemId}/bids` | REST 出价（压测/备用路径） |
| HTTP GET | `/api/v1/items/{itemId}/bids` | 出价历史（游标分页） |
| WebSocket | `/ws/rooms/{roomId}?userId={userId}` | 加入房间长连接 |

---

## 8. 限流与降级策略

### 8.1 应用层限流

- 单用户对单商品 2 秒内只能出价 1 次
- 使用原子命令：`SET auction:item:{item_id}:ratelimit:{user_id} 1 EX 2 NX`
- 只有 Key 不存在时设置成功，存在时拒绝出价
- 多实例共享计数器

### 8.2 WebSocket 重连限流

- 单用户 1 秒内最多重连 3 次
- 超出限制的连接直接拒绝握手

### 8.3 降级策略

| 场景 | 降级动作 |
|---|---|
| Redis 不可用 | 新出价返回服务不可用；落槌任务完全转由 DB 兜底扫描驱动 |
| MySQL 写入慢或失败 | Redis 已接受的出价进入补偿队列，后台重试写库；前端广播延迟增大 |
| 单个房间连接数过高 | 限制单房间最大在线人数（如 5000），超出返回排队提示并关闭新连接 |
| 重连风暴 | 限流 + room_state 本地缓存 + Redis Pipeline 批量读取 |

---

## 9. 性能目标与 SLO

| 指标 | 目标 | 测量边界 |
|---|---|---|
| 单商品出价 QPS | ≥ 5,000 | 单实例，REST 出价接口，并发 1,000，失败率 < 0.1% |
| 出价 P99 延迟 | < 50ms | 服务端处理耗时，不含网络传输 |
| WebSocket 推送延迟 | < 1s | 从服务端收到出价到向房间内所有在线观众推送完成，单房间 ≤ 1,000 人在线 |
| 断线重连恢复时间 | < 1s | 从 WebSocket 建连完成到收到 room_state 快照 |
| 并发出价一致性 | 100% | 100 并发同时向同一商品出价，最高价正确 |

---

## 10. 可观测性

### 10.1 日志

- 所有 HTTP 请求带 `request_id`
- 关键路径日志：出价、落槌、WS 连接/断开、状态变更
- 错误日志包含业务错误码

### 10.2 指标

| 指标 | 类型 | 说明 |
|---|---|---|
| auction_bid_qps | Counter | 出价 QPS |
| auction_bid_latency | Histogram | 出价处理延迟 |
| auction_ws_connections | Gauge | 当前 WS 连接数 |
| auction_room_online | Gauge | 房间内在线人数 |
| auction_hammer_total | Counter | 落槌次数 |
| auction_bid_rejected_total | Counter | 出价拒绝次数（按原因） |
| auction_pending_hammer | Gauge | 待落槌任务数 |
| auction_ws_send_queue | Gauge | WS 发送队列长度 |

### 10.3 健康检查

- `/health`：HTTP 存活检查
- `/ready`：依赖检查（MySQL、Redis 可连接）

---

## 11. Docker 编排说明

基础服务：`app` + `mysql` + `redis`。

Redis 必须开启过期键通知：
```
notify-keyspace-events Ex
```

MySQL 通过 `deployments/migrations/` 下的 SQL 文件初始化，生产环境不使用 GORM AutoMigrate。

---

## 12. 关键设计决策（ADR）

### ADR-1：同时提供 WebSocket 和 REST 出价路径

- **决策：** WebSocket 为主路径，REST 为压测/备用路径
- **原因：** WebSocket 与实时广播同连接，体验好；REST 满足 PRD 性能验收口径，便于压测
- **代价：** 需要维护两套接入层，但共享同一出价核心

### ADR-2：Redis Lua 脚本处理出价原子性

- **决策：** 所有出价竞争通过 Redis Lua 脚本串行化
- **原因：** 简单、可靠、避免分布式锁复杂度
- **代价：** 单商品出价性能受 Redis 单线程限制，但足以满足 5,000 QPS 目标

### ADR-3：Pub/Sub 模式订阅做实时广播

- **决策：** 使用 Redis `psubscribe auction:room:*` 向房间内所有在线用户广播
- **原因：** 支持多实例水平扩展，无需动态管理频道
- **代价：** 实例会收到所有房间消息，需本地按 room_id 过滤；消息不保证必达

### ADR-4：过期键通知 + 高频兜底扫描做落槌

- **决策：** 使用 Redis 过期键事件通知触发落槌，配合 5 秒周期兜底扫描
- **原因：** 事件驱动精度高，兜底扫描保证最终一致
- **代价：** 需要 Redis 开启 `notify-keyspace-events Ex`，兜底扫描会带来少量 DB 查询

### ADR-5：本期同步写 MySQL，暂不上 Kafka

- **决策：** 出价成功后同步写 MySQL
- **原因：** 本期不做支付结算，直接写库更简单可靠；二期再引入 Kafka 异步落库 + 对账
- **代价：** 写库延迟会增加出价响应时间，需控制事务粒度；MySQL 故障时需要补偿机制

### ADR-6：跨实例单连接约束通过 Redis Hash + Pub/Sub 实现

- **决策：** 使用 Redis Hash 记录 userId → conn_id@instance_id，新连接通过 Pub/Sub 通知旧实例踢线
- **原因：** 保证多实例下用户同一房间只有一个连接
- **代价：** 增加一次 Redis 写和一次 Pub/Sub 消息

---

## 13. 与前序项目的对比

| 维度 | 秒杀 | IM | 实时拍卖 |
|---|---|---|---|
| 通信 | HTTP 短连接 | WebSocket 长连接 | HTTP + WebSocket 混合 |
| 并发竞争 | 库存扣减 | 无 | 出价竞争（价高者得） |
| 实时推送 | 无 | 点对点/群组 | 房间广播（一对多） |
| 状态机 | 简单 | 连接状态 | 商品 5 状态严格流转 |
| 定时任务 | 无 | 心跳 | 倒计时落槌 + 兜底扫描 |
| 数据一致性 | 库存最终一致 | 消息可达 | 出价不串价、落槌不遗漏 |

---

## 14. 目录结构规划

```
auction/
├── cmd/auction/main.go          # 入口
├── internal/
│   ├── config/                   # 配置加载
│   ├── middleware/               # Gin 中间件
│   ├── room/                     # 房间管理 + WebSocket Hub
│   ├── item/                     # 商品管理 + 状态机 + 落槌
│   ├── bid/                      # 出价核心（Lua + 限流）
│   ├── repository/               # 数据访问层
│   └── model/                    # GORM 模型
├── pkg/
│   ├── response/                 # 统一响应格式（复用 seckill）
│   ├── logger/                   # 结构化日志
│   └── redis/                    # Redis 客户端封装
├── api/openapi.yaml              # 接口契约
├── docs/
│   ├── REQUIREMENTS.md           # PRD
│   └── ARCHITECTURE.md           # 本文档
├── configs/config.yaml           # 配置文件
├── deployments/
│   ├── docker-compose.yaml
│   └── migrations/init.sql
└── test/
    ├── e2e/
    └── bench/
```
