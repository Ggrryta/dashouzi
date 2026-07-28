# IM 聊天系统 - 5 天开发排期计划

> 每天 4-6 小时。采用垂直切片方式：每天交付一个可验证的功能增量。
> 三个 IM 节点本质是同一套代码部署三次，所以只需写一个 IM Node 服务。

---

## 架构决策

- **Gateway 和 IM Node 是否分开？** 分开。Gateway 负责 HTTP 认证 + 一致性哈希分配节点，IM Node 负责 WebSocket + 消息逻辑。职责清晰，各自可独立扩缩。
- **三个 IM Node 写三份代码？** 不。同一份二进制 + 不同端口 + 不同配置文件，Docker Compose 启动三个实例。
- **Redis Pub/Sub vs Kafka 分工？** Pub/Sub 用于实时跨节点消息转发（毫秒级），Kafka 用于离线消息异步投递（可靠性优先）。

---

## Day 1：项目骨架 + 七容器编排 + WebSocket 握手

**目标：** Gateway 返回 JWT + 节点地址，IM Node 接受 WebSocket 连接，ping/pong 互通。

### 上午（3h）：骨架 + Docker

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 1.1 | `go mod init im`，Gateway 空壳 | `cmd/gateway/main.go` | `/api/v1/ping` 返回 pong |
| 1.2 | IM Node 空壳 + gorilla/websocket | `cmd/imnode/main.go` | WebSocket 升级成功 |
| 1.3 | 配置模块：Gateway + IM Node 各自 config | `internal/config/` | 两个进程有独立配置 |
| 1.4 | 复用 pkg/（errcode, response, logger, jwt） | `pkg/` | 编译通过 |
| 1.5 | 用户注册/登录 + JWT（Gateway） | `internal/handler/auth.go` | curl 注册 + 登录拿 token |

### 下午（3h）：Docker + WebSocket

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 1.6 | Dockerfile（多阶段，IM Node 通用） | `Dockerfile` | 构建成功 |
| 1.7 | docker-compose.yml（7 容器） | 4 个服务 | `docker compose up -d` 全绿 |
| 1.8 | WebSocket 升级 + ping/pong | IM Node handler | 客户端连上 + 自动回复 pong |
| 1.9 | Gateway 返回节点地址 | `/api/v1/login` 加 node_address 字段 | 登录后知道连哪个节点 |
| 1.10 | Makefile | 支持 make up/down/test | 一键启动 |

### Day 1 验收

```bash
# 登录获取 token + 节点地址
curl -X POST /api/v1/login -d '{"username":"a","password":"x"}'
# {"token":"eyJ...","node":"ws://localhost:8081/ws"}

# WebSocket 连接 IM Node
websocat "ws://localhost:8081/ws?token=eyJ..."
# 连接成功，心跳 pong 正常

# Gateway ping
curl /api/v1/ping  # pong
```

**坑点：**
- gorilla/websocket 的 CheckOrigin 必须处理，否则跨域拒绝
- Gateway 的一致性哈希在 Day 4 才实现，Day 1 先用轮询返回固定节点
- 7 个容器端口：gateway:8080, node1:8081, node2:8082, node3:8083, mysql:3308, redis:6380, kafka:9093

---

## Day 2：WebSocket 连接管理 + 在线状态

**目标：** 多用户同时在线，心跳保活，ConnManager 管理连接生命周期。

### 上午（3h）：ConnManager

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 2.1 | Client 结构体：conn + Send channel + UserID | `internal/ws/client.go` | 读写 goroutine 分离 |
| 2.2 | ConnManager：Add/Remove/Get | `internal/ws/manager.go` | 按 user_id 查连接 |
| 2.3 | 读 goroutine：接收客户端消息 | client 内 | 消息写入 Hub |
| 2.4 | 写 goroutine：从 Send channel 取消息推送 | client 内 | 并发安全写 WebSocket |
| 2.5 | 登录时注册到 ConnManager | IM Node handler | 同用户重复登录 → 踢掉旧连接 |

### 下午（3h）：心跳 + 在线状态

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 2.6 | 心跳：30s Ping，90s 无 Pong → 断连 | client 内 | 不活跃连接自动清理 |
| 2.7 | 在线状态写入 Redis | `internal/ws/status.go` | `IM:online` Hash 正确 |
| 2.8 | 离线检测定时任务 | 独立 goroutine | 60s 无心跳 → 标记离线 |
| 2.9 | 断线重连（客户端侧） | 客户端模拟 | 指数退避重连 |
| 2.10 | 单元测试：ConnManager Add/Remove/Get | `internal/ws/manager_test.go` | 覆盖率 > 80% |

### Day 2 验收

```bash
# 用户 A 连 Node1
curl /api/v1/login → token_A, node ws://node1:8081/ws
websocat ws://node1:8081/ws?token=token_A

# 用户 B 连 Node1
curl /api/v1/login → token_B, node ws://node1:8081/ws
websocat ws://node1:8081/ws?token=token_B

# 检查 Redis 在线状态
redis-cli HGETALL im:online
# user_a → node-1
# user_b → node-1

# 心跳保活：30s 内不发送消息，连接不断开
# 手动断网 → 90s 后 Redis 中状态变离线
```

---

## Day 3：单节点消息 + ACK + 持久化

**目标：** 同一节点内两个用户可以互发消息，有 ACK 回执，消息存 MySQL。

### 上午（3h）：消息核心

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 3.1 | 消息 Model + MySQL 建表 | `internal/model/message.go` | GORM AutoMigrate |
| 3.2 | Message Repository | `internal/repository/message.go` | Create + FindByUser + UpdateStatus |
| 3.3 | 消息格式定义：`{type, to, content, msg_id}` | `internal/ws/protocol.go` | JSON 序列化/反序列化 |
| 3.4 | Hub：消息分发中心 | `internal/ws/hub.go` | 接收消息 → 查 ConnManager → 推送 |
| 3.5 | 消息先存 DB 再推送 | Hub 内 | 存库成功后才推，status=pending |

### 下午（3h）：ACK + 测试

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 3.6 | ACK 机制：收方回复 ACK → 更新 status=delivered | Hub 内 | DB 状态变更 |
| 3.7 | 发送方收到送达回执 | Hub 内 | 推送 `{type:"ack", msg_id:"..."}` |
| 3.8 | 同节点单聊：A → B 在同一节点 | E2E | 消息送达 + ACK |
| 3.9 | 对不在线用户发消息 → pending 不丢 | 单元测试 | 消息存 DB，等待离线推送 |
| 3.10 | 单元测试：Hub 消息路由 + ACK | `internal/ws/hub_test.go` | 覆盖率 > 80% |

### Day 3 验收

```bash
# A 和 B 都在 Node1 在线
# A 发送: {"type":"msg","to":<B_id>,"content":"hello","msg_id":"uuid-1"}
# B 收到: {"type":"msg","from":<A_id>,"content":"hello","msg_id":"uuid-1"}
# B 自动回复 ACK: {"type":"ack","msg_id":"uuid-1"}
# A 收到: {"type":"ack","msg_id":"uuid-1","status":"delivered"}

# 验证 MySQL
mysql> SELECT * FROM im_messages WHERE msg_id='uuid-1';
# status: delivered
```

---

## Day 4：多节点部署 + Redis Pub/Sub 跨节点路由 + 一致性哈希

**目标：** A 在 Node1，B 在 Node2，消息通过 Redis Pub/Sub 转发送达。

### 上午（3h）：Redis Pub/Sub + 节点发现

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 4.1 | 节点注册：启动时写入 Redis | `internal/ws/registry.go` | `im:nodes` Hash 正确 |
| 4.2 | 订阅自己频道：`im:node:{node_id}` | `internal/ws/pubsub.go` | 收到其他节点转发的消息 |
| 4.3 | 发送方查接收方节点 → 不是本节点 → Publish | Hub 内修改 | 跨节点消息正确投递 |
| 4.4 | 接收方节点收到 Pub/Sub → 本节点 ConnManager 推送 | pubsub handler | 消息送达对端用户 |
| 4.5 | 跨节点 ACK：ACK 也走 Pub/Sub 回传给发送方节点 | Hub 内 | ACK 回执跨节点 |

### 下午（3h）：一致性哈希 + 多节点联调

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 4.6 | 一致性哈希：Gateway 分配用户到节点 | `pkg/hash/consistent.go` | 同用户始终返回同节点 |
| 4.7 | 虚拟节点 150 个 | consistent hash 实现 | 负载均衡 |
| 4.8 | 多节点 E2E：A 连 Node1，B 连 Node2，互发消息 | 手动测试 | 消息送达 + ACK 返回 |
| 4.9 | 节点宕机测试：停掉 Node2 → A 连到 Node1 | 手动测试 | 一致性哈希漂移 |
| 4.10 | 单元测试：一致性哈希 + Pub/Sub | `pkg/hash_test.go` | 覆盖率 > 80% |

### Day 4 验收

```bash
# Gateway 分配：A → Node1, B → Node2（一致性哈希）
# A 连 ws://node1:8081, B 连 ws://node2:8082

# A 发消息给 B
# → Node1 查 online → B 在 Node2
# → Node1 Publish "im:node:node-2" {msg}
# → Node2 收到 → 推给 B
# → B 回 ACK

# 停掉 Node2
docker stop im_node_2
# → B 断连 → 重连 Gateway → 一致性哈希漂移 → 连到 Node3
# → A 再发消息 → 查 online → B 在 Node3 → Pub/Sub 转发 → OK
```

---

## Day 5：离线消息 + Kafka + E2E + 文档

**目标：** 用户离线时消息不丢，上线后通过 Kafka 异步投递。

### 上午（3h）：离线消息

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 5.1 | Kafka Producer：离线消息投递 | `pkg/kafka/producer.go` | 复用秒杀系统的 kafka-go |
| 5.2 | 发消息时，接收方不在线 → 投递到 Kafka topic | Hub 内 | `im:offline:{user_id}` |
| 5.3 | Kafka Consumer：用户上线时消费离线消息 | `pkg/kafka/consumer.go` | 启动独立 goroutine |
| 5.4 | 离线消息推送后 → ACK → 更新 DB status | consumer handler | 消息状态从 pending → delivered |
| 5.5 | 离线消息去重：msg_id 幂等 | consumer 内 | 重复消息不推送两次 |

### 下午（3h）：E2E + README

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 5.6 | E2E 测试脚本 | `scripts/e2e_test.sh` | 注册→登录→WS 连接→消息→ACK |
| 5.7 | 离线消息 E2E：用户发消息时 B 不在线 | E2E shell | B 上线后收到 |
| 5.8 | 跨节点 E2E：A 在 Node1，B 在 Node2 | E2E shell | 消息通过 Pub/Sub 送达 |
| 5.9 | README.md | 项目根目录 | 完整文档 |
| 5.10 | GitHub push | 公开仓库 | 别人能跑起来 |

### Day 5 验收

```bash
# E2E 全链路
./scripts/e2e_test.sh
# ✅ 注册 + 登录 + 分配节点
# ✅ WebSocket 连接 + 心跳
# ✅ 同节点消息 + ACK
# ✅ 跨节点消息（Redis Pub/Sub）
# ✅ 离线消息（B 断线 → A 发消息 → B 重连 → 收到）

# 验证 Kafka topic
kafka-topics --list
# im:offline:user_a ✅
```

---

## 整体时间线

```
Day1         Day2          Day3         Day4           Day5
骨架 7容器   连接管理      单节点消息    多节点+PubSub   离线+Kafka+E2E
  ██          ██           ██           ██              ██
  └──── docker compose 从 Day1 跑到 Day5 ────────────────┘
```

---

## 与服务端 HTTP 项目的差异

| 能力 | 博客/秒杀 | IM |
|---|---|---|
| 项目结构 | 1 个 cmd/server | **cmd/gateway + cmd/imnode 两个独立进程** |
| 协议 | HTTP RESTful | HTTP(Gateway) + **WebSocket(IM Node)** |
| 连接 | 短连接，无状态 | **长连接，有状态**（连接绑定到节点） |
| 多实例 | 无状态水平扩展 | **有状态**（用户连哪个节点由哈希决定） |
| 跨进程通信 | 不需要 | **Redis Pub/Sub** 跨节点消息 |

---

## 验收检查清单

- [ ] 7 个容器一键启动，healthcheck 全绿
- [ ] Gateway 登录返回 JWT + 分配的节点地址
- [ ] WebSocket 连接 + ping/pong 心跳
- [ ] ConnManager 正确管理连接生命周期
- [ ] 在线状态写入 Redis，心跳超时自动清理
- [ ] 同节点两个用户互发消息 + ACK 回执
- [ ] 消息写入 MySQL，ACK 后 status=delivered
- [ ] Redis Pub/Sub 跨节点消息路由
- [ ] 一致性哈希分配用户到节点，宕机漂移
- [ ] 离线用户收到 Kafka 异步投递的离线消息
- [ ] E2E 全链路通过
