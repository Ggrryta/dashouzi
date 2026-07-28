# 秒杀系统 - 5 天开发排期计划

> 每天 4-6 小时。每个任务有明确产出物和验收标准。
> 比博客系统少 2 天：项目骨架和安全相关的代码（JWT/RBAC/DFA）可复用博客模式。

---

## 整体依赖图

```
MySQL 建表 ──→ Seckill CRUD ──→ 库存预热 ──→ Lua 扣减
                                          │
                                          ├──→ 限流
                                          │
                                          └──→ Kafka Producer ──→ Consumer 写订单
                                                                    │
                                                                    └──→ 对账
```

---

## Day 1：项目骨架 + Docker 五容器编排

**目标：** 5 个服务一键启动，MySQL 自动建表。

### 上午（3h）

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 1.1 | `go mod init seckill`，Gin 空壳 | `cmd/server/main.go` | `/api/v1/ping` 返回 pong |
| 1.2 | 配置模块 | `internal/config/config.go` | MySQL DSN + Redis Addr + Kafka Broker |
| 1.3 | GORM + MySQL 连接 + AutoMigrate | `internal/model/*.go` | 启动后 3 张表自动创建 |
| 1.4 | 复用博客的 response/errcode/logger | 直接复制 `pkg/` | 编译通过 |
| 1.5 | 路由骨架 + Recovery + Logger 中间件 | `internal/router/` | 复用博客模式 |

### 下午（3h）

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 1.6 | Dockerfile（多阶段，CGO_ENABLED=0） | `Dockerfile` | 构建成功 |
| 1.7 | docker-compose.yml | 5 个服务：app + MySQL + Redis + ZK + Kafka | `docker compose up -d` 全绿 |
| 1.8 | MySQL init.sql | `scripts/init.sql` | 建表 + 插入测试数据 |
| 1.9 | Kafka topic 自动创建脚本 | `scripts/create-topics.sh` | `seckill.orders` topic 创建 |
| 1.10 | Makefile | `Makefile` | `make up/down/test` |

### Day 1 验收

```bash
docker compose up -d
docker compose ps  # 5 个服务全 Running + healthy
curl localhost:8080/api/v1/ping  # {"code":0,"data":"pong"}
mysql -h 127.0.0.1 -P 3306 -u seckill -p -e "SHOW TABLES"
# seckill_sessions, seckill_items, seckill_orders
```

**坑点：**
- Kafka 启动需要等 ZK 完全就绪，`depends_on` + `healthcheck` 不够，可能需要 `wait-for-it.sh`
- MySQL 首次启动较慢（30s+），healthcheck 要设足够 timeout
- Kafka advertised.listeners 必须配成 `kafka:9092`（容器内部）和 `localhost:9092`（宿主机压测用）

---

## Day 2：秒杀场次 & 商品管理

**目标：** 创建秒杀场次、添加秒杀商品、库存预热到 Redis。

### 上午（3h）

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 2.1 | Session + Item + Order Model | `internal/model/` 补充 | GORM tag 正确 |
| 2.2 | Session Repository | `internal/repository/session.go` | CRUD + 按时间范围查 |
| 2.3 | Item Repository | `internal/repository/item.go` | CRUD + 按 session 查列表 |
| 2.4 | Session + Item Service | `internal/service/` | 创建/列表/详情 |

### 下午（3h）

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 2.5 | Session + Item Handler | `internal/handler/` | RESTful 接口 |
| 2.6 | 库存预热逻辑 | `internal/service/preheat.go` | 秒杀开始前 load 库存到 Redis |
| 2.7 | 注册路由 | router 更新 | curl 验证 |

### Day 2 验收

```bash
# 创建秒杀场次
curl -X POST /api/v1/seckill/sessions \
  -d '{"name":"双11秒杀","start_time":"...","end_time":"..."}'

# 添加秒杀商品
curl -X POST /api/v1/seckill/items \
  -d '{"session_id":1,"title":"iPhone","price":1.00,"origin_price":9999,"total_stock":100}'

# 预热库存
curl -X POST /api/v1/seckill/items/1/preheat

# 验证 Redis
redis-cli GET seckill:stock:1  # 100
```

**坑点：**
- 库存预热必须在校验「秒杀是否已开始」之后，避免提前暴露库存
- Redis 库存 key 要设 TTL？不建议——对账时需要拿 Redis 剩余库存做比对

---

## Day 3：核心秒杀逻辑

**目标：** Lua 脚本原子扣库存 + 一人一单 + 令牌桶限流。

### 上午（3h）

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 3.1 | Lua 脚本编写 | `scripts/seckill.lua` | EVAL 执行返回 1/0/-1 |
| 3.2 | Redis Lua 测试（Go 单元） | `internal/service/seckill_test.go` | mock Redis 验证三种返回 |
| 3.3 | Seckill Service | `internal/service/seckill.go` | LoadScript + Execute |
| 3.4 | 令牌桶限流中间件 | `internal/middleware/ratelimit.go` | 复用博客模式 + Redis 版 |

### 下午（3h）

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 3.5 | Seckill Handler | `internal/handler/seckill.go` | POST /buy 接口 |
| 3.6 | 查询结果接口 | GET /result/:item_id | 返回是否抢到 |
| 3.7 | 联调：预热 → 抢购 → 查结果 | Docker 全链路 | 库存正确扣减 |

### Day 3 验收

```bash
# 预热 100 件库存
# 用户 A 抢购
curl -X POST /api/v1/seckill/buy -H "X-User-Id: 1" \
  -d '{"item_id":1}'
# {"code":0,"data":{"result":"success"}}

# 用户 A 重复抢
# {"code":0,"data":{"result":"already_bought"}}

# 99 个不同用户抢完 → 库存归零
redis-cli GET seckill:stock:1  # 0

# 再抢
# {"code":0,"data":{"result":"sold_out"}}
```

**坑点：**
- Lua 脚本用 EVALSHA 缓存 SHA 值，减少网络传输
- 限流用 Redis INCR + EXPIRE，不要用 Go 内存（多实例不共享）
- `X-User-Id` header 模拟用户（没有 JWT，简化设计）

---

## Day 4：Kafka 异步下单

**目标：** 扣减成功后投递 Kafka → Consumer 消费写 MySQL。

### 上午（3h）

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 4.1 | Kafka Producer 封装 | `pkg/kafka/producer.go` | Send 成功，topic 自动创建 |
| 4.2 | 抢购成功 → 投递 Kafka | 改 Seckill Service | Redis 扣减后发送消息 |
| 4.3 | Kafka Consumer 封装 | `pkg/kafka/consumer.go` | Consumer Group + 手动 commit |

### 下午（3h）

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 4.4 | Consumer 写 MySQL | consumer goroutine | 订单写入 seckill_orders 表 |
| 4.5 | Order Repository | `internal/repository/order.go` | Create + FindByUser |
| 4.6 | 消费失败重试 | consumer 内死信逻辑 | 3 次重试后进死信 topic |
| 4.7 | 全链路联调 | Docker 全链路 | Redis→Kafka→MySQL 一条消息不丢 |

### Day 4 验收

```bash
# 抢购 10 次（不同用户）
# → Consumer 消费 10 条
# → MySQL 中 10 条订单
mysql> SELECT COUNT(*) FROM seckill_orders WHERE item_id = 1;
# 10

# 验证 offset 提交
kafka-consumer-groups --bootstrap-server localhost:9092 \
  --group seckill-order-consumer --describe
# CURRENT-OFFSET = LOG-END-OFFSET，无 lag
```

**坑点：**
- Consumer 必须在**成功写 MySQL 后**才 commit offset，否则 crash 丢消息
- 同一 user_id 的消息进同一 partition（key=user_id），保证同一用户订单顺序
- 重复消费幂等：UNIQUE(user_id, item_id) 兜底

---

## Day 5：压测 + 监控 + 文档

**目标：** wrk 压测验证 QPS ≥ 10,000，不超卖。

### 上午（3h）

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 5.1 | wrk Lua 脚本 | `scripts/bench/seckill.lua` | 多用户并发抢购 |
| 5.2 | 压测：1000 并发，10w 请求 | 压测报告 | QPS 数据 |
| 5.3 | 验证不超卖 | Redis 库存 + MySQL 订单数对比 | 订单数 ≤ 初始库存 |
| 5.4 | 验证一人一单 | Redis Set 大小 = MySQL 订单数 | 相等 |

### 下午（3h）

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 5.5 | 对账脚本 | `scripts/reconcile.sh` | Redis 库存 + MySQL 订单数一致 |
| 5.6 | README.md | 项目根目录 | 克隆 → 启动 → 压测 三步 |
| 5.7 | E2E 测试脚本 | `scripts/e2e_test.sh` | 全链路 curl 测试 |
| 5.8 | GitHub push | 公开仓库 | 别人能跑起来 |

### Day 5 验收

```bash
# 压测
wrk -t12 -c400 -d30s --latency \
  -s scripts/bench/seckill.lua \
  http://localhost:8080/api/v1/seckill/buy

# 不超卖验证
redis-cli GET seckill:stock:1           # 剩余库存
mysql -e "SELECT COUNT(*) FROM seckill_orders WHERE item_id=1"  # 实际售出
# 剩余库存 = 初始库存 - 实际售出 ✅
```

---

## 整体时间线

```
Day1      Day2        Day3        Day4        Day5
骨架      商品管理    秒杀核心    Kafka      压测
五容器    CRUD        Lua扣减    异步下单    验证
  ██       ██          ██         ██         ██
  └─── docker compose 从 Day1 一直跑到 Day5 ───┘
```

---

## 与博客系统的差异（不复用的部分）

| 能力 | 博客 | 秒杀 | 是否复用 |
|---|---|---|---|
| 项目骨架 | Gin + GORM + Viper | 同 | ✅ 复用模式 |
| 响应格式 | {code,message,data} | 同 | ✅ 直接复用 pkg/ |
| 中间件 | Recovery/Logger | Recovery/Logger/RateLimit | ✅ 复用 + 新增 |
| 测试 | mock + httptest | mock + Docker 集成 | ✅ 复用模式 |
| 数据库 | PG | MySQL | 🆕 GORM 驱动不同 |
| 认证 | JWT + RBAC | X-User-Id 模拟 | 🆕 不做认证 |
| 核心逻辑 | 状态机/DFA | Lua 脚本/削峰 | 🆕 全新 |

---

## 验收检查清单

- [ ] 5 个容器一键启动，healthcheck 全绿
- [ ] Kafka topic 创建，Producer 发送 + Consumer 消费
- [ ] Lua 脚本三种返回值：成功 / 库存不足 / 已购买
- [ ] Redis 库存扣减原子性（并发场景不超卖）
- [ ] 一人一单（同一用户重复请求被拦截）
- [ ] 扣减成功 → Kafka 消息 → MySQL 订单（不丢消息）
- [ ] Consumer 手动 commit offset
- [ ] 压测 QPS ≥ 10,000
- [ ] 对账：Redis 库存 + MySQL 订单数 = 初始库存
