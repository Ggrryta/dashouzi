# 实时拍卖系统 - 测试计划

> 本文档描述如何验证系统是否满足 `docs/REQUIREMENTS.md` 中的需求。开发顺序见 `docs/DEVELOPMENT_PLAN.md`。

---

## 1. 测试金字塔与分层策略

```
        ┌─────────┐
        │  E2E    │  少量，覆盖核心用户旅程
        │  ~5%    │
       ├───────────┤
       │  集成测试  │  覆盖 Redis/MySQL/WS 交互
       │  ~15%     │
      ├─────────────┤
      │   单元测试   │  覆盖状态机、Lua、限流、repo
      │   ~80%      │
     └───────────────┘
```

### 分层定义

| 层级 | 范围 | 运行环境 | 依赖 |
|---|---|---|---|
| 单元测试 | 单个函数/方法 | 本地 `go test` | 仅依赖 mock / 内存实现 |
| 集成测试 | 多个模块协作 | Docker | 真实 MySQL + Redis |
| E2E 测试 | 完整用户旅程 | Docker | 完整服务栈 |
| 压测 | 性能与并发 | Docker | 完整服务栈 + 压测工具 |

> 规则：**集成测试、E2E 测试、压测禁止本地跑，必须走 Docker。**

---

## 2. 测试工具

| 用途 | 工具 | 说明 |
|---|---|---|
| Go 单元测试 | `go test` + `testify` | 标准测试框架 |
| Redis 单测 | `miniredis` | 内存 Redis，用于 Lua 脚本测试 |
| HTTP 测试 | `httptest` | Gin handler 测试 |
| WebSocket 测试 | `gorilla/websocket` client | E2E/集成测试 |
| MySQL 集成测试 | Docker Compose MySQL | 真实数据库 |
| 压测 | `wrk` + Lua 脚本 | REST 出价接口压测 |
| 覆盖率 | `go test -cover` | 单元测试覆盖率阈值 60% |

---

## 3. 单元测试计划

### 3.1 房间模块（`internal/room`）

| 测试目标 | 场景 | 断言 |
|---|---|---|
| 创建房间 | 合法参数 | 返回房间，status=online |
| 创建房间 | 名称为空 | 返回参数错误 |
| 查询房间 | 存在/不存在 | 返回正确/404 |
| 房间列表 | 分页 | 返回 items + nextCursor |

### 3.2 商品模块（`internal/item`）

| 测试目标 | 场景 | 断言 |
|---|---|---|
| 创建商品 | 合法参数 | 返回商品，status=pending |
| 创建商品 | start_time ≥ end_time | 返回参数错误 |
| 状态机 | pending → live | 转移成功 |
| 状态机 | live → closing → sold | 转移成功 |
| 状态机 | live → pending | 非法转移，返回错误 |
| 删除商品 | pending 状态 | 删除成功 |
| 删除商品 | live 状态 | 返回状态不允许 |
| 自动开始 | 到达 start_time | status 变为 live，Redis 被初始化 |

### 3.3 出价模块（`internal/bid`）

| 测试目标 | 场景 | 断言 |
|---|---|---|
| Lua 脚本 | 首次出价 ≥ start_price | 成功，current_price 更新 |
| Lua 脚本 | 首次出价 < start_price | 失败，返回 -1 |
| Lua 脚本 | 后续出价 ≥ current + increment | 成功 |
| Lua 脚本 | 后续出价过低 | 失败 |
| Lua 脚本 | 并发 100 出价 | 最高价正确，bid_count 正确 |
| 限流器 | 2 秒内两次出价 | 第二次被拒绝 |
| 限流器 | 2 秒后再次出价 | 允许 |
| 出价 service | 商品非 live | 返回状态错误 |
| 出价 service | MySQL 写失败 | 进入补偿队列，不广播 |

### 3.4 WebSocket 房间模块（`internal/room`）

| 测试目标 | 场景 | 断言 |
|---|---|---|
| Hub | 用户加入 | online 集合包含 userId |
| Hub | 用户断开 | online 集合移除 userId |
| Hub | 新连接踢旧连接 | 旧连接被关闭 |
| 心跳 | 30s ping / 60s 超时 | 超时后连接关闭 |
| 消息处理 | 非法 JSON | 返回 error，保持连接 |

### 3.5 Repository 层（`internal/repository`）

| 测试目标 | 场景 | 断言 |
|---|---|---|
| RoomRepo | CRUD | 数据库状态正确 |
| ItemRepo | 按状态筛选 | 返回结果正确 |
| BidRepo | 游标分页 | 返回 items + nextCursor |
| BidRepo | 查询最大出价 | 返回最高价记录 |

---

## 4. 集成测试计划

集成测试在 Docker 中运行，覆盖真实 MySQL + Redis 交互。

### 4.1 房间与商品集成

```go
// 测试：创建房间 → 创建商品 → 查询商品列表
func TestRoomAndItemLifecycle(t *testing.T) {
    // 1. 创建房间
    // 2. 创建 3 个商品
    // 3. 查询房间内商品列表
    // 4. 断言商品数量、状态
}
```

### 4.2 Redis Lua 出价集成

```go
// 测试：商品 live 后，通过 Lua 脚本出价
func TestBidLuaIntegration(t *testing.T) {
    // 1. 创建商品并设置为 live
    // 2. 用户 A 出价 100000（首次，等于起拍价）
    // 3. 用户 B 出价 100500（过低，应失败）
    // 4. 用户 B 出价 101000（合法，应成功）
    // 5. 断言 Redis 中 current_price=101000, top_bidder=B
    // 6. 断言 MySQL 有 2 条成功出价记录
}
```

### 4.3 WebSocket 广播集成

```go
// 测试：多个 WS 客户端加入，出价后都收到 bid_update
func TestWSBroadcast(t *testing.T) {
    // 1. 创建房间和 live 商品
    // 2. 用户 A、B 建立 WS 连接并 join
    // 3. 用户 A 出价
    // 4. 断言 A 和 B 都在 1s 内收到 bid_update
}
```

### 4.4 落槌集成

```go
// 测试：商品到达 end_time 后自动落槌
func TestHammerIntegration(t *testing.T) {
    // 1. 创建商品，start_time=now, end_time=now+2s
    // 2. 等待商品变为 live
    // 3. 用户出价
    // 4. 等待到达 end_time + 5s
    // 5. 断言商品状态=sold，winner_id=出价人
    // 6. 断言 Redis 落槌键被清理
}
```

### 4.5 断线重连集成

```go
// 测试：断线后重连恢复 room_state
func TestReconnectIntegration(t *testing.T) {
    // 1. 用户 A 加入房间
    // 2. 用户 A 出价
    // 3. 断开连接
    // 4. 重新连接并 join
    // 5. 断言 1s 内收到 room_state，current_price 正确
    // 6. 断言 online 集合中 userId 只出现一次
}
```

---

## 5. E2E 测试计划

E2E 测试覆盖完整用户旅程，在 Docker 中运行。

### 5.1 主流程 E2E

```go
func TestFullAuctionFlow(t *testing.T) {
    // 1. 卖家创建房间
    // 2. 卖家在房间创建商品（起拍价 100000，最小加价 1000，5 秒后结束）
    // 3. 等待商品自动变为 live
    // 4. 用户 A、B 通过 WS 加入房间
    // 5. 用户 A 出价 100000
    // 6. 用户 B 出价 100500（过低，应被拒绝）
    // 7. 用户 B 出价 101000
    // 8. 等待商品落槌
    // 9. 断言所有用户收到 hammer 事件，status=sold，winner=用户 B，currentPrice=101000
    // 10. 查询出价历史，断言 2 条记录
    // 11. 查询商品详情，断言 winner_id 一致
}
```

### 5.2 流拍 E2E

```go
func TestFailedAuctionFlow(t *testing.T) {
    // 1. 创建房间和商品（5 秒后结束）
    // 2. 等待商品自动变为 live
    // 3. 用户 A 通过 WS 加入房间（不出价）
    // 4. 等待商品落槌
    // 5. 断言 hammer 事件 status=failed，winner_id=null
}
```

### 5.3 断线重连 E2E

```go
func TestReconnectE2E(t *testing.T) {
    // 1. 创建房间和商品
    // 2. 用户 A 加入房间
    // 3. 用户 A 出价
    // 4. 断开连接
    // 5. 用户 A 重新加入房间
    // 6. 断言收到 room_state，状态正确
    // 7. 用户 A 再次出价（应成功）
}
```

### 5.4 多商品隔离 E2E

```go
func TestMultiItemIsolation(t *testing.T) {
    // 1. 创建房间
    // 2. 创建商品 A 和商品 B
    // 3. 等待两者都变为 live
    // 4. 用户 A 对商品 A 出价
    // 5. 断言商品 A current_price 更新，商品 B 不变
    // 6. 断言房间内用户只收到商品 A 的 bid_update
}
```

---

## 6. 压测计划

### 6.1 REST 出价接口压测

```bash
wrk -t8 -c1000 -d30s --latency \
  -s test/bench/bid.lua \
  http://localhost:8080/api/v1/items/1/bids
```

**压测脚本 `test/bench/bid.lua`：**

```lua
math.randomseed(os.time())

request = function()
    local userId = math.random(1, 1000)
    local amount = 100000 + math.random(0, 1000) * 100
    local body = string.format('{"amount":%d}', amount)
    return wrk.format(
        "POST",
        wrk.path,
        {
            ["Content-Type"] = "application/json",
            ["X-User-Id"] = tostring(userId)
        },
        body
    )
end
```

**验收阈值：**

| 指标 | 目标 |
|---|---|
| QPS | ≥ 5,000 |
| P99 延迟 | < 50ms |
| 失败率 | < 0.1% |

### 6.2 并发一致性压测

```bash
# 100 个并发用户同时向同一商品出价
go run test/bench/concurrent_bid.go --item-id=1 --users=100
```

**验证：**

```sql
-- MySQL 最大出价
SELECT MAX(amount) FROM bids WHERE item_id = 1;

-- Redis 最高价
redis-cli GET auction:item:1:current_price

-- 两者必须相等
```

### 6.3 WebSocket 广播压测

```bash
# 模拟 1000 个 WS 连接加入房间，持续出价，统计消息送达率
go run test/bench/ws_broadcast.go --room-id=1 --connections=1000
```

**验收阈值：**

| 指标 | 目标 |
|---|---|
| 消息送达率 | ≥ 99.9% |
| 推送延迟 P99 | < 1s |
| 系统无崩溃 | 内存/CPU 稳定 |

---

## 7. 对账方案

### 7.1 实时对账

每次落槌后执行：

```sql
-- MySQL 端最大出价
SELECT MAX(amount) AS max_amount, bidder_id
FROM bids
WHERE item_id = ?;
```

```bash
# Redis 端
redis-cli GET auction:item:{item_id}:current_price
redis-cli GET auction:item:{item_id}:top_bidder
```

**断言：** MySQL 最大出价金额 = Redis current_price，MySQL 出价人 = Redis top_bidder。

### 7.2 周期对账

```bash
# 对账脚本 scripts/reconcile.sh
# 遍历所有 sold/failed 商品，检查：
# 1. MySQL 最大出价 = Redis current_price（若 Redis 仍存在）
# 2. MySQL 出价记录数 = Redis bid_count
# 3. 所有 live 商品都有对应的 Redis timer
```

### 7.3 补偿机制

若对账发现 Redis 有但 MySQL 缺失的出价记录：
1. 从 Redis 读取出价信息
2. 补写入 MySQL bids 表
3. 记录补偿日志

---

## 8. 测试红线

以下问题出现时，测试不通过：

1. **出价串价**：并发出价后最高价不正确
2. **落槌遗漏**：live 商品到达 end_time 后未落槌
3. **消息丢失**：在线用户未收到出价/落槌广播（在可接受阈值内）
4. **断线重连失败**：重连后 1 秒内未收到 room_state
5. **数据不一致**：Redis 最高价 ≠ MySQL 最大出价
6. **限流失效**：2 秒内同一用户多次出价成功
7. **非法状态转移**：状态机被绕过

---

## 9. 测试执行命令

```bash
# 单元测试
go test ./...

# 单元测试 + 覆盖率
go test -cover ./...

# 集成测试（Docker）
make test-integration

# E2E 测试（Docker）
make test-e2e

# REST 出价压测
make bench-bid

# 并发一致性压测
make bench-concurrent

# 对账
make reconcile

# 全部测试
make test-all
```

---

## 10. 与开发计划的对应关系

| 开发阶段 | 测试计划对应 |
|---|---|
| Day 0 | 环境测试：Docker 启动、health/ready 接口 |
| Day 1 | 单元测试：room/item/repo；集成测试：房间商品生命周期 |
| Day 2 | 单元测试：Lua、限流、WS Hub；集成测试：出价、广播 |
| Day 3 | 集成测试：落槌、出价历史；E2E：主流程 |
| Day 4 | 集成测试：断线重连；单元测试覆盖率达标 |
| Day 5 | E2E 完整覆盖、压测、对账、红线检查 |
