# 实时 IM 聊天系统 - 架构设计文档

## 1. 项目定位

博客是 Level 1「单体夯实」，秒杀是 Level 2「高并发入门」，IM 是 Level 3「真正的分布式系统」。

**核心目标：** 多节点水平扩展 + WebSocket 长连接管理 + 跨节点消息路由。

## 2. 技术栈

| 层 | 选型 | 为什么 |
|---|---|---|
| 语言 | Go 1.23+ | goroutine 天然适合高并发连接 |
| WebSocket | gorilla/websocket | Go 最成熟的 WS 库 |
| 网关 | Gin + gorilla/websocket | HTTP 认证 + WS 升级同一端口 |
| 消息路由 | Redis 7 Pub/Sub | 跨节点实时转发，无需额外中间件 |
| 数据库 | MySQL 8.0 | 消息历史持久化 |
| 缓存 | Redis 7 | 在线状态 + 消息路由 |
| 消息队列 | Kafka | 离线消息异步投递 + 多端同步 |
| 负载均衡 | 一致性哈希 | 用户黏性：同一用户固定连同一节点 |
| 容器 | Docker Compose | 7+ 容器编排 |

## 3. 整体架构

```
                     ┌───────────────────┐
                     │      Gateway       │  ← 认证 + 一致性哈希分配节点
                     │   (Gin + WS up)    │     返回目标节点地址给客户端
                     └──────┬──┬──┬──────┘
                            │  │  │
                ┌───────────┘  │  └───────────┐
                ▼              ▼              ▼
           ┌─────────┐   ┌─────────┐   ┌─────────┐
           │ IM Node1 │   │ IM Node2 │   │ IM Node3 │
           │         │◄──┼─────────┼──►│         │
           │ conns:  │   │ conns:  │   │ conns:  │
           │ user A  │   │ user B  │   │ user C  │
           └────┬────┘   └────┬────┘   └────┬────┘
                │              │              │
                └──────────────┼──────────────┘
                               │
                    ┌──────────┴──────────┐
                    │       Redis          │
                    │  • Pub/Sub 消息路由   │
                    │  • 在线状态管理       │
                    │  • 节点注册/发现      │
                    └──────────┬──────────┘
                               │
                    ┌──────────┴──────────┐
                    │        MySQL         │
                    │  • 消息历史存储       │
                    │  • 用户信息           │
                    └──────────┬──────────┘
                               │
                    ┌──────────┴──────────┐
                    │        Kafka         │
                    │  • 离线消息异步投递    │
                    │  • 多端消息同步        │
                    └─────────────────────┘
```

## 4. 核心流程

### 4.1 用户上线

```
1. 客户端 HTTP POST /login → Gateway 返回 JWT + 分配的 IM 节点地址
2. 客户端 WebSocket 连接分配的 IM 节点
3. IM 节点鉴权（JWT 校验）
4. 用户连接注册到本地 connManager
5. IM 节点发布上线通知 → Redis Pub/Sub → 其他节点感知
6. IM 节点拉取 Kafka 中的离线消息 → 推送给用户
```

### 4.2 单聊消息（同节点）

```
A(在 Node1) ──发送消息──→ Node1 ──查 B 的本地连接──→ 找到 B, 直接推送
                                                      │
                                                      └──→ 本地未找到, 走跨节点流程
```

### 4.3 单聊消息（跨节点）

```
A(在 Node1) 发送消息给 B(在 Node2):

1. A → Node1: 发送 {"to":"B", "content":"hello"}
2. Node1 查本地 connManager → B 不在 Node1
3. Node1 查在线状态 → B 在 Node2
4. Node1 → Redis Pub/Sub → Publish "im:node:node-2" {msg}
5. Node2 订阅收到消息 → 查本地 connManager → 找到 B → 推送
```

### 4.4 消息可靠性（ACK 机制）

```
A 发消息给 B:
  A → Node1: msg {id: uuid}
  Node1 → DB: 存储消息（status: pending）
  Node1 → B: 推送消息（通过 Redis Pub/Sub 路由）
  B → Node2: ACK {msg_id}
  Node2 → Redis Pub/Sub: Publish ACK
  Node1 收到 ACK → DB: 更新消息 status: delivered
  Node1 → A: 送达回执
```

### 4.5 离线消息

```
B 离线时:
  A → Node1: msg to B
  Node1 → DB: 存储（status: pending）
  Node1 → A: "消息已发送"（不等待送达）

B 上线时:
  Node2 检测 B 上线
  Node2 → DB: SELECT * FROM messages WHERE to_user=B AND status='pending'
  Node2 → B: 批量推送离线消息
  B → Node2: ACK each msg
  Node2 → DB: UPDATE status='delivered'
```

## 5. 多节点关键技术

### 5.1 一致性哈希（用户黏性）

```go
// 不用普通 hash(userID) % N（节点增减时大量用户漂移）
// 用一致性哈希环：节点增减只影响相邻节点的用户

ring := consistent.New()
ring.Add("node-1:8080")
ring.Add("node-2:8080")
ring.Add("node-3:8080")

targetNode := ring.Get(userID)  // 同一 userID 始终返回同一节点
```

**虚拟节点：** 每个物理节点映射 150 个虚拟节点到哈希环上，负载更均衡。

### 5.2 Redis Pub/Sub 跨节点路由

```go
// 每个 IM 节点启动时：
//   1. 注册节点信息到 Redis
//   2. 订阅自己的频道

// 节点注册
redis.HSet("im:nodes", "node-1", "192.168.1.10:8081")
redis.HSet("im:nodes", "node-2", "192.168.1.11:8082")

// 订阅
pubsub := redis.Subscribe("im:node:node-1")
go func() {
    for msg := range pubsub.Channel() {
        // 收到其他节点转发来的消息
        handleIncomingMessage(msg)
    }
}()

// 发布（发给其他节点）
targetNode := resolveNode(recipientID)
redis.Publish("im:node:"+targetNode, serializedMsg)
```

### 5.3 在线状态管理

```go
// Redis Hash: {user_id → node_id, last_heartbeat}
redis.HSet("im:online", "user_1", "node-1")
redis.HSet("im:online", "user_2", "node-2")

// 心跳保活
go func() {
    ticker := time.NewTicker(10 * time.Second)
    for range ticker.C {
        redis.HSet("im:online", userID, nodeID)
        redis.HSet("im:heartbeat", userID, time.Now().Unix())
    }
}()

// 离线检测（独立定时任务）
// 每 30s 扫描 heartbeat，超过 60s 未更新 → 认为离线 → 从 online 中删除
```

## 6. 数据模型

```sql
-- 消息表
CREATE TABLE im_messages (
    id          BIGINT AUTO_INCREMENT PRIMARY KEY,
    msg_id      VARCHAR(36)  NOT NULL UNIQUE,  -- UUID，幂等 + ACK 用
    from_user   BIGINT       NOT NULL,
    to_user     BIGINT       NOT NULL,
    content     TEXT         NOT NULL,
    msg_type    VARCHAR(16)  NOT NULL DEFAULT 'text',  -- text/image/file
    status      VARCHAR(16)  NOT NULL DEFAULT 'pending', -- pending/delivered/read
    created_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_from_user (from_user, created_at),
    INDEX idx_to_user_status (to_user, status, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 会话列表（每个对话的最新一条消息）
CREATE TABLE im_conversations (
    id           BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_a       BIGINT       NOT NULL,
    user_b       BIGINT       NOT NULL,
    last_msg     TEXT,
    last_msg_at  TIMESTAMP,
    unread_count INT          NOT NULL DEFAULT 0,
    UNIQUE KEY uk_pair (LEAST(user_a, user_b), GREATEST(user_a, user_b))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

## 7. Redis Key 设计

| Key | 类型 | 说明 |
|---|---|---|
| `im:nodes` | Hash | 节点注册：node_id → address |
| `im:online` | Hash | 在线状态：user_id → node_id |
| `im:heartbeat` | Hash | 心跳时间：user_id → timestamp |
| `im:node:{node_id}` | Pub/Sub Channel | 节点间消息转发 |
| `im:ack:{msg_id}` | Pub/Sub Channel | ACK 回执通道 |

## 8. WebSocket 连接管理

```go
type ConnManager struct {
    mu    sync.RWMutex
    conns map[uint64]*Client  // user_id → WS connection
}

type Client struct {
    UserID   uint64
    Conn     *websocket.Conn
    Send     chan []byte       // 发送队列（避免并发写）
    LastPing time.Time
}
```

**关键设计：**
- 每个连接一个 Send channel + 写 goroutine，避免并发写 WebSocket
- 读 goroutine 持续读取客户端消息
- 心跳：30s 一次 Ping，90s 无 Pong 断开
- 断线重连：客户端指数退避重连（1s → 2s → 4s → 8s → 16s → max 30s）

## 9. 接口设计

| 协议 | 路径 | 说明 |
|---|---|---|
| HTTP POST | `/api/v1/login` | 登录，返回 JWT + 分配的节点地址 |
| HTTP POST | `/api/v1/register` | 注册 |
| WebSocket | `ws://{node}:{port}/ws?token={jwt}` | IM 长连接 |
| WebSocket | 发送消息：`{"type":"msg","to":2,"content":"hello","msg_id":"uuid"}` |
| WebSocket | ACK：`{"type":"ack","msg_id":"uuid"}` |
| WebSocket | 心跳：`{"type":"ping"}` → 回复 `{"type":"pong"}` |

## 10. Docker 编排

```yaml
services:
  gateway:      # 入口：登录 + 节点分配
  im-node-1:    # IM 实例1 (端口 8081)
  im-node-2:    # IM 实例2 (端口 8082)
  im-node-3:    # IM 实例3 (端口 8083)
  mysql:        # 消息持久化
  redis:        # 在线状态 + Pub/Sub 路由
  kafka:        # 离线消息 + 多端同步
```

7 个容器，3 个 IM 节点水平扩展。

## 11. 与秒杀系统的对比

| 维度 | 秒杀 | IM |
|---|---|---|
| 通信 | HTTP 短连接 | **WebSocket 长连接** |
| 节点数 | 1 个 app | **1 gateway + 3 IM 节点** |
| 连接数 | 无状态 | 10,000+ 并发长连接 |
| Redis | Lua 库存 | **Pub/Sub 跨节点路由** |
| Kafka | 异步下单 | **离线消息 + 多端同步** |
| 新算法 | — | **一致性哈希** |
| 新挑战 | 原子扣减 | **连接管理 + 心跳 + 断线重连** |

## 12. 面试高频问题

| 问题 | 答案要点 |
|---|---|
| 用户怎么分配到节点？ | 一致性哈希，同一用户固定连同一节点 |
| 跨节点消息怎么送达？ | Redis Pub/Sub，每个节点订阅自己频道 |
| 消息不丢怎么保证？ | ACK 机制 + 消息先存 DB + 状态机 |
| 节点挂了怎么办？ | 一致性哈希漂移 + WebSocket 断线重连 |
| 为什么用 Redis Pub/Sub 而不是 Kafka？ | Pub/Sub 是实时推送，Kafka 是异步消费。IM 消息必须毫秒级送达 |
