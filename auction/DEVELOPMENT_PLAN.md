# 实时拍卖会系统 - 5 天开发排期计划

> 每天 4-6 小时。每个任务有明确产出物和验收标准。
> 复用秒杀/IM 的项目骨架（Gin + GORM + Viper + 中间件），省 1-2 天基础设施搭建。

---

## 整体依赖图

```
MySQL 建表 ──→ 拍卖会 CRUD ──→ 状态机 ──→ WS Hub + 房间管理
                                    │
                                    ├──→ Lua 原子出价 + 限流 + Pub/Sub 广播
                                    │
                                    ├──→ 倒计时落槌（过期键 + 兜底扫描）
                                    │
                                    ├──→ Kafka 出价日志异步落库
                                    │
                                    └──→ 支付结算（状态机 + 幂等 + 超时取消）
```

---

## Day 1：项目骨架 + Docker 五容器编排

**目标：** 5 个服务一键启动，MySQL 自动建表，WS 连接能 ping/pong。

### 上午（3h）

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 1.1 | `go mod init auction`，Gin 空壳 | `cmd/auction/main.go` | `/api/v1/ping` 返回 pong |
| 1.2 | 配置模块 | `internal/config/config.go` | MySQL DSN + Redis Addr + Kafka Broker |
| 1.3 | GORM + MySQL 连接 + AutoMigrate | `internal/model/*.go` | 启动后 3 张表自动创建 |
| 1.4 | 复用秒杀/IM 的 response/errcode/logger | 复制 `pkg/` | 编译通过 |
| 1.5 | 路由骨架 + Recovery + Logger 中间件 | `internal/router/` | 复用模式 |

### 下午（3h）

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 1.6 | WS 底层封装（gorilla/websocket） | `pkg/ws/conn.go` | Upgrade + Read/Write 封装 |
| 1.7 | Dockerfile（多阶段，CGO_ENABLED=0） | `Dockerfile` | 构建成功 |
| 1.8 | docker-compose.yml | 5 服务：app + MySQL + Redis + ZK + Kafka | `docker compose up -d` 全绿 |
| 1.9 | MySQL init.sql | `deployments/migrations/init.sql` | 建表 |
| 1.10 | Kafka topic 自动创建脚本 | `scripts/create-topics.sh` | `auction.bids` + `auction.events` topic |
| 1.11 | Makefile | `Makefile` | `make up/down/test` |

### Day 1 验收

```bash
docker compose up -d
docker compose ps  # 5 个服务全 Running + healthy
curl localhost:8080/api/v1/ping  # {"code":0,"data":"pong"}
mysql -h <mysql-host> -P 3306 -u auction -p -e "SHOW TABLES"
# auctions, bids, auction_orders
```

**坑点：**
- Redis 必须配 `notify-keyspace-events Ex`（Day 3 倒计时要用），写进 compose command
- Kafka advertised.listeners 配 `kafka:9092`（容器内）和 `localhost:9092`（宿主机压测）
- MySQL 首次启动慢（30s+），healthcheck timeout 设够
- 配置地址用容器服务名（mysql/redis/kafka），禁止 127.0.0.1/0.0.0.0

---

## Day 2：拍卖会管理 + WebSocket 房间 + 状态机

**目标：** 创建拍卖会、查询列表/详情，WS 连接能加入/离开房间，状态机强校验。

### 上午（3h）

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 2.1 | Auction + Bid + Order Model | `internal/model/` | GORM tag 正确 |
| 2.2 | Auction Repository | `internal/auction/repo.go` | CRUD + 按状态/时间查 |
| 2.3 | 状态机 DFA | `internal/auction/state.go` | 合法/非法转移校验 |
| 2.4 | Auction Service | `internal/auction/service.go` | 创建/列表/详情 + 状态校验 |

### 下午（3h）

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 2.5 | Auction Handler | `internal/auction/handler.go` | RESTful 接口 |
| 2.6 | WS Hub（房间管理） | `internal/room/hub.go` | 注册/注销连接 + 房间广播 |
| 2.7 | WS 连接生命周期 | `internal/room/conn.go` | join/leave/heartbeat |
| 2.8 | WS 路由 `/ws/auction/:id` | router 注册 | 浏览器能连上 + ping/pong |
| 2.9 | 在线成员 Set 维护 | Redis `auction:online:{id}` | join 时 SADD，leave 时 SREM |

### Day 2 验收

```bash
# 创建拍卖会
curl -X POST /api/v1/auctions \
  -d '{"title":"名画拍卖","start_price":1000,"bid_increment":100,"start_time":"...","end_time":"..."}'

# 列表/详情
curl /api/v1/auctions
curl /api/v1/auctions/1

# WebSocket 连接（wscat 测试）
wscat -c "ws://localhost:8080/ws/auction/1?token=fake"
> {"type":"ping"}
< {"type":"pong"}

# 验证在线成员
redis-cli SMEMBERS auction:online:1
```

**坑点：**
- 状态机校验必须在 Service 层，非法转移直接返回业务错误码
- WS 连接断开必须从 Hub 注销，否则内存泄漏 + 在线成员数不准
- Hub 用 `map[roomID]map[conn]*Conn` + `sync.RWMutex` 管理，广播时持锁遍历
- 心跳：客户端 30s 一次 ping，服务端 60s 无心跳主动断开

---

## Day 3：Lua 原子出价 + 限流 + Pub/Sub 广播 + 倒计时落槌

**目标：** 出价核心跑通——限流 → Lua 原子出价 → Pub/Sub 广播到房间。

**这是整个系统最关键的一天。**

### 上午（3h）

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 3.1 | Lua 出价脚本 | `scripts/bid.lua` | EVAL 执行返回 1/-1 |
| 3.2 | 出价限流（Redis INCR+EXPIRE） | `internal/bid/ratelimit.go` | 单用户 2s/次 |
| 3.3 | Bid Service（Lua + 限流） | `internal/bid/service.go` | 出价成功/过低/限流三种返回 |
| 3.4 | WS 出价消息处理 | `internal/room/bid.go` | 收到 bid 消息 → 调 Bid Service |

### 下午（3h）

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 3.5 | Pub/Sub 广播 | `internal/room/pubsub.go` | 出价成功 → Publish → 全房收到 |
| 3.6 | 倒计时键 | `internal/auction/timer.go` | live 时 SET auction:timer:{id} EX {dur} |
| 3.7 | 过期键监听 Consumer | `internal/auction/expired.go` | 收到过期事件 → 触发落槌 |
| 3.8 | 兜底扫描任务 | `internal/auction/scanner.go` | 每分钟扫过期的 live 拍卖补落槌 |
| 3.9 | 落槌流程 | `internal/auction/hammer.go` | live→closing→sold/failed + 推送 |

### Day 3 验收

```bash
# 拍卖进入 live（手动触发或等 start_time）
# WS 连接两个客户端到房间 1
# 客户端 A 出价 1100
> {"type":"bid","amount":1100}
# 客户端 A 收到成功，客户端 B 收到 bid_update
< {"type":"bid_update","price":1100,...}

# 客户端 B 出价 1050（低于当前价）
< 出价过低

# 客户端 B 2 秒内连出 2 次
< 第二次被限流

# 验证不串价
redis-cli GET auction:current:1  # 1100

# 倒计时到期 → 自动落槌
< {"type":"hammer","auction_id":1,"status":"sold",...}
```

**坑点：**
- Lua 脚本用 EVALSHA 缓存 SHA 值，减少网络传输
- 限流用 Redis INCR + EXPIRE，不用 Go 内存（多实例不共享）
- Pub/Sub 消息不持久化，重启丢失——落槌通知这种关键消息要同时写 DB 状态
- 过期键通知可能因 Redis 重启丢失，必须配兜底扫描任务
- 兜底扫描用 `SELECT * FROM auctions WHERE status='live' AND end_time < NOW()`，幂等处理

---

## Day 4：出价历史 + Kafka 异步落库 + 支付结算

**目标：** 出价日志异步落 MySQL，支付结算状态机 + 幂等 + 超时取消。

### 上午（3h）

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 4.1 | Kafka Producer 封装 | `pkg/kafka/producer.go` | Send 成功 |
| 4.2 | 出价成功 → 投递 Kafka | 改 Bid Service | Redis 出价后发消息到 auction.bids |
| 4.3 | Kafka Consumer 封装 | `pkg/kafka/consumer.go` | Consumer Group + 手动 commit |
| 4.4 | Consumer 写出价记录 | consumer goroutine | 写入 bids 表 |
| 4.5 | 出价历史查询（游标分页） | `internal/auction/history.go` | 按 created_at 游标翻页 |

### 下午（3h）

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 4.6 | Order Model + Repository | `internal/payment/repo.go` | CRUD |
| 4.7 | 支付状态机 | `internal/payment/state.go` | pending_payment→paid/cancelled |
| 4.8 | 支付接口（模拟回调） | `internal/payment/handler.go` | POST /orders/:id/pay |
| 4.9 | 支付幂等 | order_id 去重 | 重复支付不重复扣 |
| 4.10 | 支付超时定时任务 | `internal/payment/scanner.go` | 30 分钟未付 → cancelled + 拍卖 failed |
| 4.11 | 落槌后生成订单 | 改 hammer 流程 | sold 时创建 pending_payment 订单 |

### Day 4 验收

```bash
# 出价后，Kafka 消费落库
mysql> SELECT COUNT(*) FROM bids WHERE auction_id=1;
# = 出价成功次数

# 出价历史游标分页
curl "/api/v1/auctions/1/bids?cursor=...&limit=20"

# 拍卖落槌 → 生成订单
mysql> SELECT * FROM auction_orders WHERE auction_id=1;
# status=pending_payment

# 支付（模拟回调）
curl -X POST /api/v1/orders/1/pay
# status=paid → 拍卖 status=settled

# 重复支付（幂等）
curl -X POST /api/v1/orders/1/pay
# 仍返回成功，不重复扣

# 超时未付（等 30 分钟或改 expire_at 测试）
# 订单 → cancelled，拍卖 → failed
```

**坑点：**
- Consumer 必须在成功写 MySQL 后才 commit offset，否则 crash 丢消息
- 出价日志幂等：用 bid 的唯一标识（如 redis incr 的 bidcount）做去重 key
- 支付幂等：UNIQUE(auction_id) 保证一个拍卖一个订单，支付状态机保证不重复付
- 超时扫描幂等：只处理 `pending_payment AND expire_at < NOW()`，处理完改状态
- 游标分页用 `WHERE auction_id=? AND created_at < ? ORDER BY created_at DESC LIMIT ?`

---

## Day 5：E2E 测试 + 压测 + 文档

**目标：** 全链路 E2E，出价压测 QPS ≥ 5,000，WS 送达率验证，文档齐全。

### 上午（3h）

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 5.1 | E2E 脚本：创建→出价→落槌→支付 | `test/e2e/full_flow_test.go` | Docker 内全链路通过 |
| 5.2 | 并发出价压测 | `test/bench/bid_test.go` | 不串价 |
| 5.3 | WS 送达率测试 | `test/bench/ws_test.go` | N 连接收到的 bid_update 数正确 |
| 5.4 | wrk 出价压测 | `scripts/bench/bid.lua` | QPS ≥ 5,000，P99 < 50ms |
| 5.5 | 不串价验证 | 对账脚本 | Redis 最高价 = MySQL 最大出价 |

### 下午（3h）

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 5.6 | 出价日志不丢验证 | 对账脚本 | Kafka 投递数 = MySQL 记录数 |
| 5.7 | 状态机全覆盖测试 | E2E | 所有合法/非法转移 |
| 5.8 | README.md | 项目根目录 | 克隆→启动→测试三步 |
| 5.9 | RESUME.md | 项目根目录 | 收获总结 |
| 5.10 | GitHub push | 公开仓库 | 别人能跑起来 |

### Day 5 验收

```bash
# E2E
docker compose run e2e go test ./test/e2e/ -v
# ✅ 创建拍卖 → live → 出价 → 落槌 → 支付 → settled

# 压测
wrk -t8 -c200 -d30s --latency \
  -s scripts/bench/bid.lua \
  http://localhost:8080/api/v1/auctions/1/bid
# Requests/sec: 6000+ ✅
# P99 Latency: 30ms ✅

# 不串价
redis-cli GET auction:current:1
mysql -e "SELECT MAX(amount) FROM bids WHERE auction_id=1"
# 两者相等 ✅
```

---

## 整体时间线

```
Day1      Day2        Day3        Day4        Day5
骨架      拍卖会管理   出价核心    落库+支付   E2E+压测
五容器    WS房间      Lua出价     Kafka      验证
  ██       ██          ██         ██         ██
  └─── docker compose 从 Day1 一直跑到 Day5 ───┘
```

---

## 与前序项目的复用关系

| 能力 | 来源 | 是否复用 |
|---|---|---|
| 项目骨架 | 秒杀/IM | ✅ Gin + GORM + Viper 模式 |
| 响应格式 | 秒杀 | ✅ `{code,message,data}` |
| 中间件 | 秒杀 | ✅ Recovery/Logger/RateLimit |
| Lua 脚本模式 | 秒杀 | ✅ 改造为出价竞争 |
| Kafka Producer/Consumer | 秒杀 | ✅ 直接复用 pkg/kafka |
| WebSocket 封装 | IM | ✅ 改造为房间广播 |
| 状态机 DFA | 博客 | ✅ 改造为拍卖状态流转 |
| 游标分页 | Feed | ✅ 改造为出价历史 |
| 数据库 | MySQL | ✅ 复用 |
| 认证 | X-User-Id 模拟 | ✅ 复用简化 |
| 支付 | 🆕 全新 | 轮廓复用秒杀订单模式 |

---

## 验收检查清单

- [ ] 5 个容器一键启动，healthcheck 全绿
- [ ] Redis 配置 `notify-keyspace-events Ex`
- [ ] Kafka topic 创建：auction.bids + auction.events
- [ ] 拍卖会 CRUD + 状态机 6 状态流转校验
- [ ] WebSocket 连接、加入房间、心跳、断开注销
- [ ] Lua 原子出价：成功 / 出价过低 / 限流 三种返回
- [ ] 并发出价不串价（最高价始终 = 实际最高出价）
- [ ] Pub/Sub 广播：出价后全房间收到 bid_update
- [ ] 倒计时落槌：过期键通知 + 兜底扫描双保险
- [ ] 落槌后生成订单 + 推送成交通知
- [ ] 出价日志 Kafka→MySQL 不丢消息
- [ ] 支付状态机 + 幂等（重复支付不重复扣）
- [ ] 支付超时取消 + 拍卖转 failed
- [ ] 出价历史游标分页
- [ ] 压测 QPS ≥ 5,000，P99 < 50ms
- [ ] 对账：Redis 最高价 = MySQL 最大出价；Kafka 投递数 = MySQL 记录数
