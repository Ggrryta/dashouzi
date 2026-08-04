# 实时拍卖系统 - 开发计划

> 本文档描述如何按阶段落地 `docs/ARCHITECTURE.md`。需求来源见 `docs/REQUIREMENTS.md`，接口契约见 `api/openapi.yaml`。

---

## 1. 开发原则

- **先基础设施，后业务代码**：Docker、配置、公共包先就绪
- **先数据层，后服务层**：model + repository 稳定后再写 handler
- **先单点正确，后水平扩展**：先在单实例跑通，再验证多实例
- **每天结束有验收**：每个阶段必须有可运行的检查点
- **禁止本地跑集成测试**：E2E 和压测必须走 Docker

---

## 2. 整体依赖图

```
Day 0: 骨架 + Docker + 公共包
    │
    ▼
Day 1: 数据层 + 房间/商品管理 + 自动开始
    │
    ▼
Day 2: WebSocket 房间 + 出价核心（Lua + 限流）
    │
    ▼
Day 3: 实时广播 + 落槌 + 出价历史
    │
    ▼
Day 4: 断线重连 + 可观测性 + 单元/集成测试
    │
    ▼
Day 5: E2E 测试 + 压测 + 修复
```

---

## 3. 复用清单

| 能力 | 来源项目 | 复用方式 | 落点 |
|---|---|---|---|
| 统一响应格式 `{code,message,data}` | 秒杀 | 复制并适配 | `pkg/response` |
| 错误码定义 | 秒杀 | 参考并扩展 | `pkg/errcode` |
| Gin Recovery/Logger 中间件 | 秒杀 | 复制 | `internal/middleware` |
| X-User-Id 透传中间件 | 秒杀/IM | 复制 | `internal/middleware/auth.go` |
| Viper 配置加载 | 秒杀/IM | 复制并改造 | `internal/config` |
| GORM + MySQL 连接 | 秒杀 | 复制 | `pkg/db` / `internal/model` |
| Redis 客户端封装 | 秒杀 | 复制并扩展 Lua 执行 | `pkg/redis` |
| WebSocket 连接封装 | IM | 改造为房间模型 | `internal/room` |
| 限流器 | 秒杀 | 改造为 Redis SET NX EX | `internal/bid/ratelimit.go` |

---

## 4. Day 0：项目骨架 + Docker + 公共包

### 目标
搭建可运行的最小工程骨架，所有服务能在 Docker 中启动。

### 任务

| 序号 | 任务 | 输出文件 |
|---|---|---|
| 0.1 | 创建项目目录结构 | `internal/`、`pkg/`、`configs/`、`deployments/`、`test/` |
| 0.2 | 编写 `go.mod` 并统一依赖版本 | `go.mod`、`go.sum` |
| 0.3 | 编写 Docker Compose 编排 | `deployments/docker-compose.yaml` |
| 0.4 | 编写 Makefile | `Makefile` |
| 0.5 | 编写配置文件 | `configs/config.yaml` |
| 0.6 | 复用统一响应格式 | `pkg/response/response.go` |
| 0.7 | 复用/编写错误码 | `pkg/errcode/errcode.go` |
| 0.8 | 复用 Gin 中间件 | `internal/middleware/logger.go`、`recovery.go`、`auth.go` |
| 0.9 | 复用 Viper 配置加载 | `internal/config/config.go` |
| 0.10 | 复用 GORM + MySQL 连接 | `pkg/db/db.go` |
| 0.11 | 复用 Redis 客户端 | `pkg/redis/redis.go` |
| 0.12 | 编写入口 `main.go` | `cmd/auction/main.go` |

### 验收标准

```bash
make up
# 检查 3 个服务 healthy
docker compose ps

# 检查健康接口
curl http://localhost:8080/health
# 期望返回 200

curl http://localhost:8080/ready
# 期望返回 200，且 MySQL/Redis 可连接
```

### 注意事项

- Redis 配置 `notify-keyspace-events Ex`
- MySQL healthcheck timeout 设 30s+
- 配置文件中地址用服务名（`mysql`、`redis`），禁用 `127.0.0.1`/`0.0.0.0`
- 入口 `main.go` 预留 graceful shutdown 钩子

---

## 5. Day 1：数据层 + 房间/商品管理 + 自动开始

### 目标
完成 MySQL 表结构、房间/商品的 CRUD、商品自动开始任务。

### 任务

| 序号 | 任务 | 输出文件 |
|---|---|---|
| 1.1 | 编写 MySQL migration | `deployments/migrations/init.sql` |
| 1.2 | 编写 GORM model | `internal/model/room.go`、`item.go`、`bid.go` |
| 1.3 | 编写 repository 接口与实现 | `internal/repository/room.go`、`item.go` |
| 1.4 | 编写房间 service | `internal/room/service.go` |
| 1.5 | 编写房间 handler | `internal/room/handler.go` |
| 1.6 | 编写商品 service（含状态机） | `internal/item/service.go` |
| 1.7 | 编写商品 handler | `internal/item/handler.go` |
| 1.8 | 编写自动开始定时任务 | `internal/item/scheduler.go` |
| 1.9 | 注册路由 | `internal/router/router.go` |
| 1.10 | 编写 repository/service 单元测试 | `*_test.go` |

### 验收标准

```bash
# 创建房间
curl -X POST http://localhost:8080/api/v1/rooms \
  -H "X-User-Id: 1" \
  -H "Content-Type: application/json" \
  -d '{"name":"测试房","description":"测试"}'

# 查询房间列表
curl "http://localhost:8080/api/v1/rooms"

# 创建商品
curl -X POST http://localhost:8080/api/v1/rooms/1/items \
  -H "X-User-Id: 1" \
  -H "Content-Type: application/json" \
  -d '{
    "title":"iPhone",
    "startPrice":100000,
    "minIncrement":1000,
    "startTime":"2026-08-04T10:00:00Z",
    "endTime":"2026-08-04T10:05:00Z"
  }'

# 等待到 start_time 后查询商品状态变为 live
curl http://localhost:8080/api/v1/items/1
```

### 关键检查点

- [ ] 房间状态为 `online`
- [ ] 商品创建后状态为 `pending`
- [ ] 到达 `start_time` 后商品自动变为 `live`
- [ ] 非法状态转移被拒绝
- [ ] 非卖家无法创建商品

---

## 6. Day 2：WebSocket 房间 + 出价核心

### 目标
完成 WebSocket 连接管理、join/heartbeat、出价核心（Lua + 限流 + REST 出价）。

### 任务

| 序号 | 任务 | 输出文件 |
|---|---|---|
| 2.1 | 编写 WebSocket 连接封装 | `internal/room/conn.go` |
| 2.2 | 编写房间 Hub（内存连接管理） | `internal/room/hub.go` |
| 2.3 | 编写 join/ping/pong 消息处理 | `internal/room/handler_ws.go` |
| 2.4 | 编写 Redis 在线集合与连接映射 | `internal/room/presence.go` |
| 2.5 | 编写跨实例踢线逻辑 | `internal/room/kick.go` |
| 2.6 | 编写 Redis Lua 出价脚本 | `scripts/bid.lua` |
| 2.7 | 编写出价 service | `internal/bid/service.go` |
| 2.8 | 编写限流器 | `internal/bid/ratelimit.go` |
| 2.9 | 编写 REST 出价 handler | `internal/bid/handler.go` |
| 2.10 | 编写 WS 出价消息处理 | `internal/bid/handler_ws.go` |
| 2.11 | 编写单元测试 | `*_test.go` |

### 验收标准

```bash
# WebSocket 连接并加入房间
wscat -c "ws://localhost:8080/ws/rooms/1?userId=42"
> {"type":"join"}
< {"type":"room_state","roomId":1,"items":[...]}

# 通过 WS 出价
> {"type":"bid","itemId":1,"amount":100000}
< {"type":"bid_update","itemId":1,"bidderId":42,"amount":100000,...}

# 通过 REST 出价
curl -X POST http://localhost:8080/api/v1/items/1/bids \
  -H "X-User-Id: 2" \
  -H "Content-Type: application/json" \
  -d '{"amount":101000}'

# 过低出价被拒绝
curl -X POST http://localhost:8080/api/v1/items/1/bids \
  -H "X-User-Id: 3" \
  -H "Content-Type: application/json" \
  -d '{"amount":100500}'
# 期望返回 409，code=300001

# 限流测试：2 秒内连续出价两次
curl -X POST ... -d '{"amount":102000}'
curl -X POST ... -d '{"amount":103000}'
# 第二次期望返回 429，code=300002
```

### 关键检查点

- [ ] WebSocket 可连接、join 后收到 room_state
- [ ] 心跳 30s ping / 60s 超时生效
- [ ] 同一用户新连接踢掉旧连接
- [ ] 有效出价成功，过低出价被拒绝
- [ ] 单用户 2 秒内限流生效
- [ ] REST 出价与 WS 出价结果一致

---

## 7. Day 3：实时广播 + 落槌 + 出价历史

### 目标
完成 Pub/Sub 广播、自动落槌、出价历史查询。

### 任务

| 序号 | 任务 | 输出文件 |
|---|---|---|
| 3.1 | 编写 Redis Pub/Sub 订阅器 | `internal/room/pubsub.go` |
| 3.2 | 编写消息分发器（bid_update / item_state / hammer） | `internal/room/dispatcher.go` |
| 3.3 | 编写出价成功后发布广播 | `internal/bid/publisher.go` |
| 3.4 | 编写状态变更广播（pending→live / live→closing） | `internal/item/broadcast.go` |
| 3.5 | 编写 Redis 倒计时键设置 | `internal/item/timer.go` |
| 3.6 | 编写过期键监听落槌 | `internal/item/hammer.go` |
| 3.7 | 编写兜底扫描任务 | `internal/item/scanner.go` |
| 3.8 | 编写出价历史 repository/service/handler | `internal/bid/history.go` |
| 3.9 | 编写单元测试与集成测试 | `*_test.go` |

### 验收标准

```bash
# 多用户加入房间
# 用户 A 出价后，用户 B 在 WS 上收到 bid_update

# 到达 end_time 后，所有在线用户收到 hammer 事件
< {"type":"hammer","itemId":1,"status":"sold","winnerId":42,"currentPrice":103000}

# 查询出价历史
curl "http://localhost:8080/api/v1/items/1/bids?size=10"
```

### 关键检查点

- [ ] 出价成功后房间内所有在线用户收到 bid_update
- [ ] 商品开始/结束时广播 item_state
- [ ] 到达 end_time 后自动落槌并广播 hammer
- [ ] 落槌后商品状态正确（sold/failed），winner_id 正确
- [ ] 兜底扫描能在 Redis 过期事件丢失时补落槌
- [ ] 出价历史按时间倒序、游标分页正确

---

## 8. Day 4：断线重连 + 可观测性 + 测试

### 目标
完成断线重连状态恢复、日志/指标/健康检查、单元与集成测试。

### 任务

| 序号 | 任务 | 输出文件 |
|---|---|---|
| 4.1 | 编写断线重连状态恢复 | `internal/room/reconnect.go` |
| 4.2 | 编写 room_state 快照组装 | `internal/room/snapshot.go` |
| 4.3 | 编写重连限流与风暴保护 | `internal/room/reconnect_limit.go` |
| 4.4 | 编写结构化日志中间件 | `pkg/logger/logger.go` |
| 4.5 | 编写 Prometheus 指标 | `internal/metrics/metrics.go` |
| 4.6 | 编写健康检查接口 | `internal/handler/health.go` |
| 4.7 | 编写单元测试（覆盖 Lua、状态机、限流、repo） | `*_test.go` |
| 4.8 | 编写 Docker 内集成测试 | `test/integration/...` |
| 4.9 | 编写 README 快速启动文档 | `README.md` |

### 验收标准

```bash
# 单元测试
go test ./...

# 集成测试（Docker 内）
make test-integration

# 断线重连测试
# 1. 用户 A 加入房间
# 2. 断开网络或关闭连接
# 3. 重新连接并 join
# 4. 1 秒内收到 room_state，状态正确

# 健康检查
curl http://localhost:8080/health
curl http://localhost:8080/ready
```

### 关键检查点

- [ ] 断线重连后 1 秒内收到 room_state
- [ ] 重连后在线人数正确，不重复计数
- [ ] 日志带 request_id
- [ ] 核心指标可采集
- [ ] 单元测试覆盖率 ≥ 60%
- [ ] 集成测试在 Docker 内通过

---

## 9. Day 5：E2E + 压测 + 修复

### 目标
完成端到端测试、性能压测、问题修复、文档收尾。

### 任务

| 序号 | 任务 | 输出文件 |
|---|---|---|
| 5.1 | 编写 E2E 全链路测试 | `test/e2e/full_flow_test.go` |
| 5.2 | 编写 wrk 压测脚本 | `test/bench/bid.lua` |
| 5.3 | 执行压测并分析结果 | 压测报告 |
| 5.4 | 修复压测中发现的问题 | 相关代码 |
| 5.5 | 编写对账脚本 | `scripts/reconcile.sh` |
| 5.6 | 更新 ARCHITECTURE.md 与接口契约中的偏差 | `docs/ARCHITECTURE.md`、`api/openapi.yaml` |
| 5.7 | 编写 CHECKLIST.md | `docs/CHECKLIST.md` |

### 验收标准

```bash
# E2E 测试
make test-e2e

# 压测出价接口
make bench-bid
# 期望：QPS ≥ 5,000，P99 < 50ms，失败率 < 0.1%

# 对账
make reconcile
# 期望：Redis 最高价 = MySQL 最大出价
#       出价记录数 = 成功出价次数
```

### 关键检查点

- [ ] E2E：创建房间 → 创建商品 → 自动开始 → WS 出价 → 落槌成交，全链路通过
- [ ] 压测：QPS ≥ 5,000，P99 < 50ms
- [ ] 并发 100 出价不串价
- [ ] 所有商品最终都落槌
- [ ] CHECKLIST.md 完成

---

## 10. 验收检查清单

### 功能验收

- [ ] 可创建房间、查询房间列表和详情
- [ ] 可创建商品、查询商品列表和详情
- [ ] 商品自动从 pending 进入 live
- [ ] WebSocket 可加入房间、心跳保活
- [ ] 有效出价成功，过低出价被拒绝
- [ ] 出价后房间内所有在线用户收到 bid_update
- [ ] 商品开始/结束时广播 item_state
- [ ] 到达 end_time 后自动落槌并广播 hammer
- [ ] 断线重连后 1 秒内恢复 room_state
- [ ] 出价历史支持游标分页

### 性能验收

- [ ] REST 出价接口 QPS ≥ 5,000
- [ ] 出价 P99 延迟 < 50ms
- [ ] 100 并发出价不串价
- [ ] 单房间 1,000 人在线时推送延迟 < 1s

### 可靠性验收

- [ ] Redis 最高价 = MySQL 最大出价
- [ ] 出价记录数 = 成功出价次数
- [ ] 所有 live 商品最终都落槌
- [ ] 断线不重号

---

## 11. 风险与应对

| 风险 | 影响 | 应对 |
|---|---|---|
| Redis Lua 脚本复杂度高 | 出价串价 | 先用 miniredis 单元测试，再集成 Redis 测试 |
| WebSocket 并发连接管理复杂 | 内存泄漏、连接泄漏 | 使用 pprof 监控，设置连接上限 |
| 落槌双保险实现复杂 | 落槌遗漏 | 单元测试 + 集成测试覆盖过期事件丢失场景 |
| 多实例 Pub/Sub 消息量大 | 网络带宽 | 使用模式订阅 + 本地过滤 |
| MySQL 写库成为瓶颈 | P99 延迟超标 | 控制事务粒度，必要时二期引入 Kafka |

---

## 12. 与测试计划的关系

详细测试策略见 `docs/TEST_PLAN.md`。本开发计划中的每天验收标准对应测试计划中的单元/集成/E2E/压测分层。

| 开发阶段 | 测试计划对应 |
|---|---|
| Day 0 | 环境测试 |
| Day 1 | 单元测试（room/item/repo） |
| Day 2 | 单元测试（bid/Lua/限流/WS） |
| Day 3 | 集成测试（广播/落槌/历史） |
| Day 4 | 集成测试（重连/可观测性） |
| Day 5 | E2E 测试 + 压测 |
