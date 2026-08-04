# 实时拍卖会系统 - 5 天测试计划

> 对标 `DEVELOPMENT_PLAN.md`，TDD 红-绿-重构循环。
> 测试金字塔：80% 单元 / 15% 集成 / 5% E2E + 压测。
> **集成测试、E2E 测试、压测禁止本地跑，必须走 Docker。**

---

## 测试金字塔分布

```
          ╱╲
         ╱  ╲         5% E2E + 压测 (~10 条)
        ╱    ╲        Docker 全链路 + wrk 5,000 QPS + WS 送达率
       ╱──────╲
      ╱        ╲      15% 集成测试 (~18 条)
     ╱          ╲     Kafka 端到端 + Redis Lua + WS 房间 Docker
    ╱────────────╲
   ╱              ╲   80% 单元测试 (~55 条)
  ╱                ╲   Mock Redis + Mock Kafka + 纯逻辑
 ╱──────────────────╲
```

---

## Day 1 测试计划：骨架 + Docker

**对应开发：** Day 1 — Go 项目初始化、五容器编排、WS 封装

| # | 测试内容 | 层级 | RED 条件 |
|---|---|---|---|
| T1.1 | 配置加载：MySQL DSN 正确拼接 | 单元 Small | config 包不存在 |
| T1.2 | 配置加载：Redis + Kafka 地址正确 | 单元 Small | — |
| T1.3 | Model 自动建表 | 集成 Medium | 表不存在 |
| T1.4 | Docker Compose 五服务 healthcheck | E2E Large | 服务未启动 |
| T1.5 | Ping 接口返回 pong | 集成 Medium | 路由未注册 |
| T1.6 | Kafka topic 自动创建 | 集成 Medium | topic 不存在 |
| T1.7 | Redis notify-keyspace-events 配置 | 集成 Medium | 配置缺失 |

### Day 1 核心用例

```go
func TestConfig_AddressesNotLocalhost(t *testing.T) {
    // 配置地址禁止用 127.0.0.1/0.0.0.0，必须用真实可路由地址
    cfg := config.Load("configs/config.yaml")
    assert.NotContains(t, cfg.Redis.Addr, "127.0.0.1")
    assert.NotContains(t, cfg.Redis.Addr, "0.0.0.0")
    assert.Contains(t, cfg.Redis.Addr, "redis")  // 容器服务名
}

func TestDockerCompose_AllServicesHealthy(t *testing.T) {
    // E2E: docker compose ps 验证 5 个服务 Status = healthy
}

func TestRedis_KeyspaceNotificationEnabled(t *testing.T) {
    // 验证 notify-keyspace-events 包含 Ex
    cfg := rdb.ConfigGet(ctx, "notify-keyspace-events").Val()
    assert.Contains(t, cfg["notify-keyspace-events"], "E")
    assert.Contains(t, cfg["notify-keyspace-events"], "x")
}
```

### Day 1 验收

```bash
go test ./pkg/... ./internal/config/... -v
# 全部 PASS ✅
```

---

## Day 2 测试计划：拍卖会管理 + WS 房间 + 状态机

**对应开发：** Day 2 — 拍卖会 CRUD、状态机、WS Hub

| # | 测试内容 | 层级 | TDD 顺序 |
|---|---|---|---|
| T2.1 | AuctionRepo.Create：创建拍卖 | 单元 Small | 先于 2.3 |
| T2.2 | AuctionRepo.FindByStatus：按状态查 | 单元 Small | — |
| T2.3 | 状态机：pending→live 合法 | 单元 Small | 先于 2.4 |
| T2.4 | 状态机：live→closing 合法 | 单元 Small | — |
| T2.5 | 状态机：closing→sold/failed 合法 | 单元 Small | — |
| T2.6 | 状态机：sold→settled 合法 | 单元 Small | — |
| T2.7 | 状态机：pending→closing 非法（拒绝） | 单元 Small | — |
| T2.8 | 状态机：settled→live 非法（拒绝） | 单元 Small | — |
| T2.9 | AuctionService.Create：结束时间必须晚于开始 | 单元 Small | 先于 2.4 |
| T2.10 | AuctionService.Create：起拍价 > 0 | 单元 Small | — |
| T2.11 | WS Hub：注册连接到房间 | 单元 Small | 先于 2.6 |
| T2.12 | WS Hub：注销连接离开房间 | 单元 Small | — |
| T2.13 | WS Hub：房间内广播消息 | 单元 Small | — |
| T2.14 | WS Hub：跨房间不串消息 | 单元 Small | — |
| T2.15 | WS 心跳：ping→pong | 集成 Medium | — |
| T2.16 | WS 心跳：60s 无心跳主动断开 | 集成 Medium | — |
| T2.17 | 在线成员 Set：join 时 SADD | 集成 Medium | — |
| T2.18 | 在线成员 Set：leave 时 SREM | 集成 Medium | — |
| T2.19 | POST /auctions：端到端创建 | E2E Large | 最后 |

### Day 2 核心用例

```go
func TestStateMachine_LegalTransition(t *testing.T) {
    sm := auction.NewStateMachine()
    assert.True(t, sm.CanTransit("pending", "live"))
    assert.True(t, sm.CanTransit("live", "closing"))
    assert.True(t, sm.CanTransit("closing", "sold"))
    assert.True(t, sm.CanTransit("sold", "settled"))
}

func TestStateMachine_IllegalTransition(t *testing.T) {
    sm := auction.NewStateMachine()
    assert.False(t, sm.CanTransit("pending", "closing"))  // 跳过 live
    assert.False(t, sm.CanTransit("settled", "live"))     // 已完成
    assert.False(t, sm.CanTransit("failed", "sold"))      // 流拍不能成交

    err := sm.Transit("pending", "closing")
    assert.Error(t, err)
}

func TestWSHub_BroadcastToRoomOnly(t *testing.T) {
    hub := room.NewHub()
    connA := mockConn("room1")
    connB := mockConn("room1")
    connC := mockConn("room2")
    hub.Register(connA)
    hub.Register(connB)
    hub.Register(connC)

    hub.Broadcast("room1", []byte(`{"type":"bid_update"}`))

    assert.Len(t, connA.Received(), 1)  // room1 收到
    assert.Len(t, connB.Received(), 1)  // room1 收到
    assert.Empty(t, connC.Received())   // room2 不收
}
```

### Day 2 验收

```bash
go test ./internal/auction/... ./internal/room/... -v
# PASS: TestStateMachine_LegalTransition
# PASS: TestStateMachine_IllegalTransition
# PASS: TestWSHub_BroadcastToRoomOnly
```

---

## Day 3 测试计划：Lua 原子出价 + 限流 + Pub/Sub + 倒计时

**对应开发：** Day 3 — 出价核心，整个系统最关键的测试日。

| # | 测试内容 | 层级 | TDD 顺序 |
|---|---|---|---|
| T3.1 | Lua 脚本：出价高于当前价 → 成功(返回1) | 单元 Small | 先于 3.3 |
| T3.2 | Lua 脚本：出价低于当前价 → 失败(返回-1) | 单元 Small | — |
| T3.3 | Lua 脚本：出价=当前价+最小加价 → 成功 | 单元 Small | — |
| T3.4 | Lua 脚本：出价<当前价+最小加价 → 失败 | 单元 Small | — |
| T3.5 | Lua 脚本：更新后最高价正确 | 单元 Small | — |
| T3.6 | Lua 脚本：更新后最高出价人正确 | 单元 Small | — |
| T3.7 | BidService.Bid：返回 success | 单元 Small | 先于 3.4 |
| T3.8 | BidService.Bid：返回 too_low | 单元 Small | — |
| T3.9 | 限流：正常频率放行 | 集成 Medium | 先于 3.4 |
| T3.10 | 限流：超频返回 429 | 集成 Medium | — |
| T3.11 | 限流：不同用户独立计数 | 集成 Medium | — |
| T3.12 | 并发安全：100 goroutine 同时出价 | 单元 Small | 最后 |
| T3.13 | 并发安全：不串价（最高价=实际最高出价） | 单元 Small | 最后 |
| T3.14 | 并发安全：只有最高出价人记录正确 | 单元 Small | 最后 |
| T3.15 | Pub/Sub：出价后广播到频道 | 集成 Medium | — |
| T3.16 | Pub/Sub：订阅者收到 bid_update | 集成 Medium | — |
| T3.17 | 倒计时：SET timer key 带 TTL | 集成 Medium | — |
| T3.18 | 过期键通知：键过期触发落槌 | 集成 Medium | — |
| T3.19 | 落槌：有出价 → sold | 集成 Medium | — |
| T3.20 | 落槌：无出价 → failed | 集成 Medium | — |
| T3.21 | 兜底扫描：过期 live 拍卖补落槌 | 集成 Medium | — |
| T3.22 | 兜底扫描：已落槌的不重复处理 | 集成 Medium | — |

### Day 3 核心用例

```go
func TestLuaScript_BidHigher_ReturnsOne(t *testing.T) {
    mr := miniredis.RunT(t)
    mr.Set("auction:current:1", "100000")  // 当前 ¥1000（分）

    result, err := svc.Bid(ctx, 1, 42, 110000)  // 出 ¥1100

    assert.NoError(t, err)
    assert.Equal(t, 1, result)
    assert.Equal(t, "110000", mr.Get("auction:current:1"))  // 更新为 ¥1100
    assert.Equal(t, "42", mr.Get("auction:bidder:1"))       // 出价人=42
}

func TestLuaScript_BidTooLow_ReturnsMinusOne(t *testing.T) {
    mr := miniredis.RunT(t)
    mr.Set("auction:current:1", "100000")

    result, _ := svc.Bid(ctx, 1, 42, 90000)  // 出 ¥900 < ¥1000

    assert.Equal(t, -1, result)
    assert.Equal(t, "100000", mr.Get("auction:current:1"))  // 不变
}

func TestConcurrent_NoPriceRace(t *testing.T) {
    mr := miniredis.RunT(t)
    mr.Set("auction:current:1", "100000")  // 起拍 ¥1000

    var wg sync.WaitGroup
    bids := []int{105000, 108000, 110000, 120000, 130000, 150000}
    success := atomic.Int64{}

    // 100 个 goroutine 用不同价格出价
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(idx int) {
            defer wg.Done()
            amount := bids[idx%len(bids)]
            result, _ := svc.Bid(ctx, 1, idx, amount)
            if result == 1 {
                success.Add(1)
            }
        }(i)
    }
    wg.Wait()

    // 最高价必须 = 最高出价 150000
    assert.Equal(t, "150000", mr.Get("auction:current:1"))
    // 成功次数 = 价格递增被超越的次数（不串价的核心验证）
    assert.True(t, success.Load() > 0)
}

func TestRateLimit_AllowThenDeny(t *testing.T) {
    mr := miniredis.RunT(t)
    rl := bid.NewRateLimiter(rdb)

    assert.True(t, rl.Allow(ctx, 1))   // 第 1 次放行
    assert.False(t, rl.Allow(ctx, 1))  // 2 秒内第 2 次拒绝
    assert.True(t, rl.Allow(ctx, 2))   // 不同用户放行
}

func TestHammer_WithBids_GoesSold(t *testing.T) {
    // 有出价的拍卖落槌 → sold + 生成订单
    mr := miniredis.RunT(t)
    mr.Set("auction:current:1", "110000")
    mr.Set("auction:bidder:1", "42")

    err := hammerSvc.Hammer(ctx, 1)

    assert.NoError(t, err)
    auction := repo.Find(1)
    assert.Equal(t, "sold", auction.Status)
    assert.Equal(t, int64(42), auction.WinnerID)
}

func TestHammer_NoBids_GoesFailed(t *testing.T) {
    // 无出价的拍卖落槌 → failed（流拍）
    mr := miniredis.RunT(t)
    // current 为空 = 无人出价

    err := hammerSvc.Hammer(ctx, 1)

    assert.NoError(t, err)
    assert.Equal(t, "failed", repo.Find(1).Status)
}
```

### Day 3 验收

```bash
go test ./internal/bid/... ./internal/auction/... -run "TestLua|TestConcurrent|TestHammer" -v
# PASS: TestLuaScript_BidHigher_ReturnsOne
# PASS: TestLuaScript_BidTooLow_ReturnsMinusOne
# PASS: TestConcurrent_NoPriceRace       ← 最关键
# PASS: TestHammer_WithBids_GoesSold
# PASS: TestHammer_NoBids_GoesFailed

go test ./... -count=1
```

---

## Day 4 测试计划：Kafka 落库 + 支付结算

**对应开发：** Day 4 — 出价日志异步落库、支付状态机、幂等、超时

| # | 测试内容 | 层级 | TDD 顺序 |
|---|---|---|---|
| T4.1 | Producer.Send：消息投递成功 | 单元 Small | 先于 4.2 |
| T4.2 | Producer.Send：分区 key 正确 | 单元 Small | — |
| T4.3 | BidService 扣减后触发 Producer.Send | 单元 Small | 先于 4.4 |
| T4.4 | Consumer.Read：拉取消息 | 单元 Small | 先于 4.4 |
| T4.5 | Consumer 解析消息写 MySQL | 单元 Small | — |
| T4.6 | Consumer 写成功后 commit offset | 单元 Small | — |
| T4.7 | Consumer 写失败后**不** commit offset | 单元 Small | — |
| T4.8 | 出价历史游标分页：第一页 | 单元 Small | — |
| T4.9 | 出价历史游标分页：翻页 | 单元 Small | — |
| T4.10 | 支付状态机：pending_payment→paid 合法 | 单元 Small | — |
| T4.11 | 支付状态机：pending_payment→cancelled 合法 | 单元 Small | — |
| T4.12 | 支付状态机：paid→paid 非法（拒绝） | 单元 Small | — |
| T4.13 | 支付幂等：重复支付不重复扣 | 单元 Small | 最后 |
| T4.14 | 超时扫描：过期 pending_payment → cancelled | 集成 Medium | — |
| T4.15 | 超时扫描：已处理的不重复 | 集成 Medium | — |
| T4.16 | 落槌生成订单：有出价 → pending_payment | 集成 Medium | — |
| T4.17 | 支付成功 → 拍卖 settled | 集成 Medium | — |
| T4.18 | Kafka → MySQL 端到端不丢 | 集成 Medium | Docker 内 |

### Day 4 核心用例

```go
func TestProducer_SendBidLog(t *testing.T) {
    mockProducer := kafka.NewMockProducer()

    err := mockProducer.Send("auction.bids", "1",
        []byte(`{"auction_id":1,"user_id":42,"amount":110000}`))

    assert.NoError(t, err)
    msgs := mockProducer.GetMessages("auction.bids")
    assert.Len(t, msgs, 1)
}

func TestConsumer_OnlyCommitAfterDBWrite(t *testing.T) {
    mockDB := &mockBidRepo{shouldFail: true}
    consumer := NewConsumer(mockDB)

    msg := &kafka.Message{Value: []byte(`{"auction_id":1,"user_id":42,"amount":110000}`)}
    err := consumer.process(msg)

    assert.Error(t, err)
    assert.False(t, consumer.wasCommitted(msg))  // 写失败不 commit
}

func TestPayment_Idempotent_DuplicatePay(t *testing.T) {
    db := setupTestDB(t)
    order := createOrder(db, 1, 42, 110000)  // pending_payment

    paySvc.Pay(ctx, order.ID)   // 第一次支付成功
    paySvc.Pay(ctx, order.ID)   // 重复支付，幂等

    updated := db.Find(order.ID)
    assert.Equal(t, "paid", updated.Status)  // 仍 paid，不报错
    assert.NotNil(t, updated.PaidAt)
}

func TestPayment_TimeoutCancel(t *testing.T) {
    db := setupTestDB(t)
    order := createOrder(db, 1, 42, 110000)
    order.ExpireAt = time.Now().Add(-time.Minute)  // 已过期
    db.Save(order)

    scanner.ScanAndCancel(ctx)

    assert.Equal(t, "cancelled", db.Find(order.ID).Status)
    // 拍卖也应转 failed
    assert.Equal(t, "failed", auctionRepo.Find(1).Status)
}
```

### Day 4 验收

```bash
go test ./internal/bid/... ./internal/payment/... ./pkg/kafka/... -v
# PASS: TestProducer_SendBidLog
# PASS: TestConsumer_OnlyCommitAfterDBWrite
# PASS: TestPayment_Idempotent_DuplicatePay
# PASS: TestPayment_TimeoutCancel

# 集成测试：真实 Kafka（Docker 内）
go test -tags=integration ./pkg/kafka/... -v
# PASS: TestKafkaEndToEnd
```

---

## Day 5 测试计划：E2E + 压测 + 对账

**对应开发：** Day 5 — 全链路 E2E、压测、文档

| # | 测试内容 | 层级 |
|---|---|---|
| T5.1 | E2E：创建→live→出价→落槌→支付→settled 全链路 | E2E Large |
| T5.2 | E2E：100 用户并发出价，Kafka→MySQL 验证 | E2E Large |
| T5.3 | E2E：WS 房间 N 连接收到 bid_update | E2E Large |
| T5.4 | E2E：状态机所有合法转移覆盖 | E2E Large |
| T5.5 | E2E：流拍（无出价落槌 → failed） | E2E Large |
| T5.6 | 压测：wrk 8 线程 200 连接 30 秒 | 性能 Large |
| T5.7 | 压测：出价 QPS ≥ 5,000 | 性能 Large |
| T5.8 | 压测：P99 < 50ms | 性能 Large |
| T5.9 | 对账：Redis 最高价 = MySQL 最大出价 | 对账 Large |
| T5.10 | 对账：Kafka 投递数 = MySQL 出价记录数 | 对账 Large |
| T5.11 | WS 送达率：N 连接全部收到广播 | 对账 Large |

### Day 5 E2E 脚本结构

```go
// test/e2e/full_flow_test.go（Docker 内运行）

func TestE2E_FullAuctionFlow(t *testing.T) {
    // 1. 创建拍卖会（start_time=now, end_time=now+60s）
    auction := createAuction(t, 1000, 100, 60*time.Second)

    // 2. 等 live 状态（或手动触发）
    waitUntilStatus(t, auction.ID, "live")

    // 3. 10 个用户并发出价
    for i := 0; i < 10; i++ {
        go bid(t, auction.ID, i+1, 1000+(i+1)*100)
    }

    // 4. 验证最高价
    current := redis.Get(fmt.Sprintf("auction:current:%d", auction.ID))
    assert.Equal(t, "190000", current)  // 最高出价 ¥1900

    // 5. 等落槌（或手动落槌）
    waitUntilStatus(t, auction.ID, "sold")

    // 6. 验证订单生成
    order := getOrder(t, auction.ID)
    assert.Equal(t, "pending_payment", order.Status)

    // 7. 支付
    pay(t, order.ID)
    waitUntilStatus(t, auction.ID, "settled")

    // 8. 对账
    maxBid := mysqlMaxBid(t, auction.ID)
    assert.Equal(t, "1900.00", maxBid)  // Redis = MySQL ✅

    kafkaCount := kafkaMessageCount(t, "auction.bids")
    dbCount := mysqlBidCount(t, auction.ID)
    assert.Equal(t, kafkaCount, dbCount)  // 不丢 ✅
}
```

### Day 5 压测脚本

```lua
-- scripts/bench/bid.lua
wrk.method = "POST"
wrk.headers["X-User-Id"] = tostring(math.random(1, 10000))
wrk.body = '{"amount":' .. tostring(100000 + math.random(1,100)*100) .. '}'
wrk.headers["Content-Type"] = "application/json"

request = function()
   wrk.path = "/api/v1/auctions/1/bid"
   return wrk.format(nil, wrk.path)
end
```

### Day 5 验收

```bash
# E2E（Docker 内）
docker compose run e2e go test ./test/e2e/ -v
# ✅ 全链路：创建→出价→落槌→支付→settled
# ✅ 并发出价不串价
# ✅ WS 房间广播送达
# ✅ 流拍处理

# 压测（必须走 Docker，禁止本地直连）
wrk -t8 -c200 -d30s --latency \
  -s scripts/bench/bid.lua \
  http://localhost:8080/api/v1/auctions/1/bid
# Requests/sec: 6000+ ✅
# P99 Latency: 30ms ✅

# 对账
redis-cli GET auction:current:1
mysql -e "SELECT MAX(amount) FROM bids WHERE auction_id=1"
# 两者相等 ✅

kafka-consumer-groups --bootstrap-server localhost:9092 \
  --group auction-bid-consumer --describe
# CURRENT-OFFSET = LOG-END-OFFSET，无 lag ✅
```

---

## 与前序项目测试的差异

| 维度 | 秒杀 | IM | 拍卖会 |
|---|---|---|---|
| 单元测试重点 | Lua 扣减、并发不超卖 | WS 连接、心跳 | **Lua 出价、并发不串价、状态机** |
| 核心断言 | 不超卖、一人一单 | 消息送达 | **不串价、状态合法、支付幂等** |
| 集成测试 | Kafka 端到端 | WS 房间 | **Kafka + Pub/Sub + 过期键 + 兜底扫描** |
| 新增工具 | miniredis | gorilla mock | **miniredis + WS mock + 过期键测试** |
| 性能测试 | wrk 10,000 QPS | 无 | **wrk 5,000 QPS + WS 送达率** |

---

## 测试工具

| 工具 | 用途 |
|---|---|
| `testify/assert` | 断言 |
| `miniredis` | 内存 Redis mock，测试 Lua 脚本 + 过期键 |
| `sarama/mocks` 或 `segmentio/kafka-go` mock | Kafka mock |
| `httptest` | HTTP handler 测试 |
| `gorilla/websocket` 客户端 | WS 房间测试 |
| `wrk` | HTTP 出价压测 |
| 自定义 Go 客户端 | WS 送达率压测 |
| `docker compose` | 集成/E2E 测试环境 |

---

## 测试统计

| 天 | 单元(Small) | 集成(Medium) | E2E(Large) | 累计 |
|---|---|---|---|---|
| Day 1 | 3 | 3 | 1 | 7 |
| Day 2 | 14 | 4 | 1 | 26 |
| Day 3 | 14 | 8 | 0 | 48 |
| Day 4 | 13 | 5 | 0 | 66 |
| Day 5 | 0 | 0 | 11 | **77** |

**金字塔比例：** 单元 44 (57%) / 集成 20 (26%) / E2E+压测 13 (17%)

> 拍卖会的集成和 E2E 比例高于秒杀，因为 WS 房间广播、过期键落槌、兜底扫描这些关键路径单靠 mock 不够，必须走 Docker 真实环境。

---

## 红线

- ❌ 不写并发测试 = 没写测试。拍卖会的核心就是并发出价不串价
- ❌ 集成测试、E2E、压测禁止本地跑，必须走 Docker
- ❌ 配置地址禁止用 127.0.0.1/0.0.0.0，必须用真实可路由地址（容器服务名）
- ❌ 过期键落槌测试必须同时验证兜底扫描（过期键通知可能丢）
- ❌ 支付测试必须验证幂等（重复支付不重复扣）+ 超时取消
- ❌ Kafka 消费测试必须验证 offset 提交逻辑（成功才提交）
- ❌ WS 测试必须验证跨房间不串消息 + 断开注销
