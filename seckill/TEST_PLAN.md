# 秒杀系统 - 5 天测试计划

> 对标 `DEVELOPMENT_PLAN.md`，TDD 红-绿-重构循环。
> 测试金字塔：80% 单元 / 15% 集成 / 5% E2E + 压测。

---

## 测试金字塔分布

```
          ╱╲
         ╱  ╲         5% E2E + 压测 (~8 条)
        ╱    ╲        Docker 全链路 + wrk 10,000 QPS
       ╱──────╲
      ╱        ╲      15% 集成测试 (~15 条)
     ╱          ╲     Kafka 端到端 + Redis Lua Docker
    ╱────────────╲
   ╱              ╲   80% 单元测试 (~45 条)
  ╱                ╲   Mock Redis + Mock Kafka + 纯逻辑
 ╱──────────────────╲
```

---

## Day 1 测试计划：骨架 + Docker

**对应开发：** Day 1 — Go 项目初始化、五容器编排

| # | 测试内容 | 层级 | RED 条件 |
|---|---|---|---|
| T1.1 | 配置加载：MySQL DSN 正确拼接 | 单元 Small | config 包不存在 |
| T1.2 | 配置加载：Redis + Kafka 地址正确 | 单元 Small | — |
| T1.3 | Model 自动建表 | 集成 Medium | 表不存在 |
| T1.4 | Docker Compose 五服务 healthcheck | E2E Large | 服务未启动 |
| T1.5 | Ping 接口返回 pong | 集成 Medium | 路由未注册 |
| T1.6 | Kafka topic 自动创建 | 集成 Medium | topic 不存在 |

### Day 1 核心用例

```go
func TestConfig_MySQLDSN(t *testing.T) {
    cfg := config.Load("config/config.yaml")
    dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True",
        cfg.DB.User, cfg.DB.Password, cfg.DB.Host, cfg.DB.Port, cfg.DB.Name)
    assert.Contains(t, dsn, "tcp(mysql:3306)")
}

func TestDockerCompose_AllServicesHealthy(t *testing.T) {
    // E2E: docker compose ps 验证 5 个服务 Status = healthy
}
```

### Day 1 验收

```bash
go test ./pkg/... ./internal/config/... -v
# 全部 PASS ✅
```

---

## Day 2 测试计划：秒杀场次 & 商品管理

**对应开发：** Day 2 — Session/Item CRUD + 库存预热

| # | 测试内容 | 层级 | TDD 顺序 |
|---|---|---|---|
| T2.1 | SessionRepo.Create：创建场次 | 单元 Small | 先于 2.2 |
| T2.2 | SessionRepo.FindActive：查当前活跃场次 | 单元 Small | — |
| T2.3 | SessionService.Create：校验时间合法性（结束 > 开始） | 单元 Small | 先于 2.4 |
| T2.4 | SessionService.Create：时间冲突检测 | 单元 Small | — |
| T2.5 | ItemRepo.Create：创建秒杀商品 | 单元 Small | 先于 2.3 |
| T2.6 | ItemRepo.FindBySession：按场次查商品列表 | 单元 Small | — |
| T2.7 | ItemService.Create：校验 session 必须存在 | 单元 Small | 先于 2.4 |
| T2.8 | ItemService.Create：价格必须小于原价 | 单元 Small | — |
| T2.9 | 库存预热：Redis SET 成功 | 集成 Medium | 先于 2.6 |
| T2.10 | 库存预热：重复预热不翻倍 | 集成 Medium | — |
| T2.11 | POST /seckill/sessions：端到端创建 | E2E Large | 最后 |
| T2.12 | POST /seckill/items：创建商品 | E2E Large | — |

### Day 2 核心用例

```go
func TestPreheat_SetsStockInRedis(t *testing.T) {
    // Arrange: mock Redis client, item with total_stock=100
    // Act: svc.Preheat(ctx, itemID)
    // Assert: redis.Set called with "seckill:stock:1", 100
}

func TestPreheat_Idempotent(t *testing.T) {
    // 重复预热不应累加库存
    svc.Preheat(ctx, 1)  // stock = 100
    svc.Preheat(ctx, 1)  // stock still = 100
    stock, _ := redis.Get("seckill:stock:1")
    assert.Equal(t, "100", stock)
}
```

### Day 2 验收

```bash
go test ./internal/service/... -v -run "TestPreheat"
# PASS: TestPreheat_SetsStockInRedis
# PASS: TestPreheat_Idempotent
```

---

## Day 3 测试计划：Lua 原子扣减 + 限流

**对应开发：** Day 3 — Lua 脚本、Seckill Service、令牌桶限流

**这是整个系统最关键的测试日。**

| # | 测试内容 | 层级 | TDD 顺序 |
|---|---|---|---|
| T3.1 | Lua 脚本：库存充足 → 扣减成功(返回1) | 单元 Small | 先于 3.3 |
| T3.2 | Lua 脚本：库存为0 → 返回0 | 单元 Small | — |
| T3.3 | Lua 脚本：用户已购买 → 返回-1 | 单元 Small | — |
| T3.4 | Lua 脚本：扣减后库存正确-1 | 单元 Small | — |
| T3.5 | Lua 脚本：用户被加入已购买集合 | 单元 Small | — |
| T3.6 | SeckillService.Buy：返回 success | 单元 Small | 先于 3.3 |
| T3.7 | SeckillService.Buy：返回 sold_out | 单元 Small | — |
| T3.8 | SeckillService.Buy：返回 already_bought | 单元 Small | — |
| T3.9 | SeckillService.Buy：验证 Lua 脚本被调用 | 单元 Small | — |
| T3.10 | 限流中间件：正常频率放行 | 集成 Medium | 先于 3.4 |
| T3.11 | 限流中间件：超频返回 429 | 集成 Medium | — |
| T3.12 | 限流中间件：不同用户独立计数 | 集成 Medium | — |
| T3.13 | 并发安全：100 goroutine 同时抢 50 库存 | 单元 Small | 最后 |
| T3.14 | 并发安全：不超卖（实际扣减=50） | 单元 Small | 最后 |
| T3.15 | 并发安全：一人一单不破 | 单元 Small | 最后 |

### Day 3 核心用例

```go
func TestLuaScript_StockAvailable_ReturnsOne(t *testing.T) {
    // 模拟 Redis：SET stock=10, user set empty
    mock := miniredis.NewMiniRedis()
    mock.Set("seckill:stock:1", "10")

    result, err := svc.ExecuteSeckill(ctx, 1, 1)
    
    assert.NoError(t, err)
    assert.Equal(t, 1, result)         // 返回 1 = 成功
    assert.Equal(t, "9", mock.Get("seckill:stock:1"))  // 库存-1
    assert.True(t, mock.SIsMember("seckill:bought:1", "1"))  // 用户进入集合
}

func TestLuaScript_AlreadyBought_ReturnsMinusOne(t *testing.T) {
    mock.SAdd("seckill:bought:1", "1")  // 用户已购买

    result, _ := svc.ExecuteSeckill(ctx, 1, 1)
    assert.Equal(t, -1, result)  // 返回 -1 = 已购买
}

func TestConcurrent_NoOversell(t *testing.T) {
    stock := 50
    mock.Set("seckill:stock:1", strconv.Itoa(stock))

    var wg sync.WaitGroup
    success := atomic.Int64{}
    
    // 100 个 goroutine 同时抢 50 库存
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(uid int) {
            defer wg.Done()
            result, _ := svc.ExecuteSeckill(ctx, uid, 1)
            if result == 1 {
                success.Add(1)
            }
        }(i)
    }
    wg.Wait()

    assert.Equal(t, int64(50), success.Load())  // 正好 50 个成功
    assert.Equal(t, "0", mock.Get("seckill:stock:1"))  // 库存归零
}

func TestSeckillHandler_Success(t *testing.T) {
    // 模拟用户 1 抢购 item 1
    w := httptest.NewRecorder()
    req := httptest.NewRequest("POST", "/buy", 
        bytes.NewBuffer([]byte(`{"item_id":1}`)))
    req.Header.Set("X-User-Id", "1")
    router.ServeHTTP(w, req)

    assert.Equal(t, 200, w.Code)
    assert.Contains(t, w.Body.String(), "success")
}
```

### Day 3 验收

```bash
go test ./internal/service/ -run "TestLua|TestConcurrent" -v
# PASS: TestLuaScript_StockAvailable_ReturnsOne
# PASS: TestLuaScript_AlreadyBought_ReturnsMinusOne
# PASS: TestConcurrent_NoOversell      ← 最关键的测试

# 验证单元测试 + 并发测试全部通过
go test ./... -count=1
```

---

## Day 4 测试计划：Kafka 异步下单

**对应开发：** Day 4 — Producer 投递 + Consumer 写 MySQL

| # | 测试内容 | 层级 | TDD 顺序 |
|---|---|---|---|
| T4.1 | Producer.Send：消息投递成功 | 单元 Small | 先于 4.1 |
| T4.2 | Producer.Send：分区 key 正确 | 单元 Small | — |
| T4.3 | SeckillService 扣减后触发 Producer.Send | 单元 Small | 先于 4.2 |
| T4.4 | Consumer.Read：拉取消息 | 单元 Small | 先于 4.3 |
| T4.5 | Consumer 解析消息并写 MySQL | 单元 Small | — |
| T4.6 | Consumer 写成功后 commit offset | 单元 Small | — |
| T4.7 | Consumer 写失败后**不** commit offset | 单元 Small | — |
| T4.8 | 重复消费幂等：UNIQUE(user_id, item_id) | 单元 Small | 最后 |
| T4.9 | Kafka → MySQL 端到端 | 集成 Medium | Docker 内 |
| T4.10 | Consumer Group 多实例 | 集成 Medium | Docker 内 |

### Day 4 核心用例

```go
func TestProducer_SendToKafka(t *testing.T) {
    mockProducer := kafka.NewMockProducer()
    
    err := mockProducer.Send("seckill.orders", "1", 
        []byte(`{"user_id":1,"item_id":1}`))
    
    assert.NoError(t, err)
    msgs := mockProducer.GetMessages("seckill.orders")
    assert.Len(t, msgs, 1)
}

func TestConsumer_OnlyCommitAfterDBWrite(t *testing.T) {
    mockDB := &mockOrderRepo{shouldFail: true}
    consumer := NewConsumer(mockDB)

    msg := &kafka.Message{Value: []byte(`{"user_id":1,"item_id":1}`)}
    err := consumer.process(msg)

    assert.Error(t, err)
    assert.False(t, consumer.wasCommitted(msg))  // 未 commit
}

func TestConsumer_Idempotent_DuplicateMessage(t *testing.T) {
    db := setupTestDB(t)
    consumer := NewConsumer(db)

    // 同一条消息消费两次
    msg := []byte(`{"user_id":1,"item_id":1}`)
    consumer.process(msg)  // 第一次成功，写入 DB
    consumer.process(msg)  // 第二次重复，UNIQUE 约束忽略

    count := db.Count("seckill_orders", "user_id=1 AND item_id=1")
    assert.Equal(t, 1, count)  // 只有一条订单
}
```

### Day 4 验收

```bash
# 单元测试：mock Kafka + mock DB
go test ./pkg/kafka/... -v
# PASS: TestProducer_SendToKafka
# PASS: TestConsumer_OnlyCommitAfterDBWrite

# 集成测试：真实 Kafka（Docker 内）
go test -tags=integration ./pkg/kafka/... -v
# PASS: TestKafkaEndToEnd
```

---

## Day 5 测试计划：压测 + 对账 + E2E

**对应开发：** Day 5 — wrk 压测、对账脚本、README

| # | 测试内容 | 层级 |
|---|---|---|
| T5.1 | E2E：预热 → 抢购 → 查结果 全链路 | E2E Large |
| T5.2 | E2E：100 用户并发抢购，Kafka → MySQL 验证 | E2E Large |
| T5.3 | 压测：wrk 12线程 400连接 30秒 | 性能 Large |
| T5.4 | 压测：QPS ≥ 10,000 | 性能 Large |
| T5.5 | 压测：P99 < 100ms | 性能 Large |
| T5.6 | 对账：Redis 库存 = 初始库存 - MySQL 订单数 | 对账 Large |
| T5.7 | 对账：Redis 已购集合大小 = MySQL 订单数 | 对账 Large |
| T5.8 | `scripts/e2e_test.sh` 一键全链路 | E2E Large |

### Day 5 E2E 脚本结构

```bash
#!/bin/bash
# scripts/e2e_test.sh

# 1. 创建秒杀场次
# 2. 添加秒杀商品（100 件库存）
# 3. 预热库存到 Redis
# 4. 10 个不同用户抢购
# 5. 验证 Redis 剩余库存 = 90
# 6. 等待 Kafka 消费
# 7. 验证 MySQL 订单数 = 10
# 8. 验证 Redis 已购集合 = 10
# 9. 同一用户重复抢购 → already_bought
# 10. 库存不足时抢购 → sold_out
```

### Day 5 验收

```bash
# E2E
./scripts/e2e_test.sh
# ✅ 预热: stock=100
# ✅ 10人抢购: Redis剩余=90
# ✅ Kafka消费: MySQL订单=10
# ✅ 对账一致

# 压测
wrk -t12 -c400 -d30s http://localhost:8080/api/v1/seckill/buy
# Requests/sec: 12000+ ✅
# P99 Latency: 45ms ✅
```

---

## 与博客测试的差异

| 维度 | 博客 | 秒杀 |
|---|---|---|
| 单元测试重点 | 状态机、DFA | **Lua 脚本、并发安全** |
| 核心断言 | 业务逻辑正确 | **不超卖、一人一单** |
| 集成测试 | PG 真实查询 | **Kafka 端到端** |
| 新增工具 | testify | **miniredis + mock kafka** |
| 性能测试 | 无 | **wrk 压测 10,000 QPS** |

---

## 测试工具

| 工具 | 用途 |
|---|---|
| `testify/assert` | 断言 |
| `miniredis` | 内存 Redis mock，测试 Lua 脚本 |
| `sarama/mocks` | Kafka mock（sarama 库自带） |
| `httptest` | HTTP handler 测试 |
| `wrk` | HTTP 压测 |
| `docker compose` | 集成测试环境 |

---

## 测试统计

| 天 | 单元(Small) | 集成(Medium) | E2E(Large) | 累计 |
|---|---|---|---|---|
| Day 1 | 3 | 2 | 1 | 6 |
| Day 2 | 8 | 2 | 2 | 18 |
| Day 3 | 10 | 3 | 0 | 31 |
| Day 4 | 8 | 2 | 0 | 41 |
| Day 5 | 0 | 0 | 8 | **49** |

**金字塔比例：** 单元 30 (61%) / 集成 9 (18%) / E2E+压测 10 (21%)

> 秒杀的集成和 E2E 比例高于博客，因为 Kafka 端到端和并发安全是核心验证点，单靠 mock 不够。

---

## 红线

- ❌ 不写并发测试 = 没写测试。秒杀的核心就是并发安全
- ❌ 压测必须走 Docker，禁止本地直连 Redis/MySQL
- ❌ Kafka 消费测试必须验证 offset 提交逻辑（成功才提交）
