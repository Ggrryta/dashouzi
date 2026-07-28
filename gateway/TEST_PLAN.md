# API 网关 - 5 天测试计划

> 对标 `DEVELOPMENT_PLAN.md`，TDD 红-绿-重构循环。
> 网关的测试有特殊性：需要 `httptest.Server` 模拟上游来验证代理转发。

---

## 测试金字塔分布

```
          ╱╲
         ╱  ╲         5% E2E (~6 条)
        ╱    ╲        gateway + 3上游 全链路
       ╱──────╲
      ╱        ╲      15% 集成测试 (~12 条)
     ╱          ╲     ReverseProxy + Auth 中间件 + Admin API
    ╱────────────╲
   ╱              ╲   80% 单元测试 (~35 条)
  ╱                ╲  4种限流算法 + 熔断器 + 路由表 + Plugin
 ╱──────────────────╲
```

---

## Day 1 测试计划：路由表 + 反向代理

**对应开发：** Day 1 — Route Table、httputil.ReverseProxy

| # | 测试内容 | 层级 | 环境 |
|---|---|---|---|
| T1.1 | RouteTable.Add：添加路由 | 单元 Small | 零依赖 |
| T1.2 | RouteTable.Match：精确匹配 | 单元 Small | 零依赖 |
| T1.3 | RouteTable.Match：前缀匹配 `/blog/api` → `/blog` | 单元 Small | 零依赖 |
| T1.4 | RouteTable.Match：无匹配返回 nil | 单元 Small | 零依赖 |
| T1.5 | ReverseProxy：转发到 upstream 并返回 | 集成 Medium | **httptest 上游** |
| T1.6 | 路径前缀剥离：`/blog/api/v1/ping` → upstream 收到 `/api/v1/ping` | 集成 Medium | **httptest 上游** |
| T1.7 | Header 透传：客户端 Header 传递给 upstream | 集成 Medium | **httptest 上游** |
| T1.8 | 上游不可达 → 返回 502 | 集成 Medium | 关掉 httptest 上游 |

### Day 1 核心用例

```go
func TestRouteTable_Match(t *testing.T) {
    table := NewRouteTable()
    table.Add(Route{Path: "/blog", Upstream: "http://bloger:8080"})
    table.Add(Route{Path: "/seckill", Upstream: "http://seckill:8081"})

    // 精确匹配
    r := table.Match("/blog")
    assert.Equal(t, "http://bloger:8080", r.Upstream)

    // 前缀匹配
    r = table.Match("/blog/api/v1/ping")
    assert.Equal(t, "http://bloger:8080", r.Upstream)

    // 无匹配
    assert.Nil(t, table.Match("/unknown"))
}

func TestReverseProxy_ForwardsToUpstream(t *testing.T) {
    // 启动假上游
    upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte(`{"code":0,"data":"pong"}`))
    }))
    defer upstream.Close()

    // 创建网关（指向假上游）
    proxy := NewProxy(upstream.URL)
    gw := httptest.NewServer(proxy.Handler())
    defer gw.Close()

    resp, _ := http.Get(gw.URL + "/api/v1/ping")
    body, _ := io.ReadAll(resp.Body)
    assert.Contains(t, string(body), "pong")
}

func TestReverseProxy_UpstreamDown_Returns502(t *testing.T) {
    proxy := NewProxy("http://localhost:19999") // 无服务
    gw := httptest.NewServer(proxy.Handler())
    defer gw.Close()

    resp, _ := http.Get(gw.URL + "/api/v1/ping")
    assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
}
```

### Day 1 验收

```bash
go test ./internal/router/... -v    # 路由表 4条
go test ./internal/proxy/... -v     # 代理 4条
```

---

## Day 2 测试计划：JWT 认证 + 四种限流算法

**对应开发：** Day 2 — Auth Middleware、Limiter 接口 + 四种实现

### JWT Auth（4 条）

| # | 测试内容 | 层级 |
|---|---|---|
| T2.1 | 有效 token → 放行 + X-User-Id Header 注入 | 集成 Medium |
| T2.2 | 无 token → 401 | 集成 Medium |
| T2.3 | 过期 token → 401 | 集成 Medium |
| T2.4 | 白名单路径免认证（`/health`） | 集成 Medium |

### 限流算法（16 条）

| # | 测试内容 | 层级 | 算法 |
|---|---|---|---|
| T2.5 | 限内放行：1 秒内 5 次 → 全 200 | 单元 Small | 固定窗口 |
| T2.6 | 超限拒绝：1 秒内 6 次 → 第 6 次 429 | 单元 Small | 固定窗口 |
| T2.7 | 窗口过期：等 1 秒后 → 恢复放行 | 单元 Small | 固定窗口 |
| T2.8 | 时间差恰好跨窗口 → 旧窗口过期 | 单元 Small | 固定窗口 |
| T2.9 | 滑动窗口：精确最近 1 秒的请求数 | 单元 Small | 滑动窗口 |
| T2.10 | 滑动窗口：旧请求过期后恢复 | 单元 Small | 滑动窗口 |
| T2.11 | 滑动窗口 vs 固定窗口：窗口边界行为不同 | 单元 Small | 对比 |
| T2.12 | 令牌桶：初始有 N 个令牌 | 单元 Small | 令牌桶 |
| T2.13 | 令牌桶：消耗完 → 拒绝 → 等 token 恢复 → 再放行 | 单元 Small | 令牌桶 |
| T2.14 | 令牌桶：并发安全（10 goroutine 同时请求） | 单元 Small | 令牌桶 |
| T2.15 | 漏桶：匀速处理，队列满溢出 | 单元 Small | 漏桶 |
| T2.16 | 4 种算法集成测试：中间件 + 真实 HTTP | 集成 Medium | 全部 |

### Day 2 核心用例

```go
func TestFixedWindow_UnderLimit(t *testing.T) {
    limiter := NewFixedWindow(5, time.Second)
    for i := 0; i < 5; i++ {
        assert.True(t, limiter.Allow("user-1"), "request %d should be allowed", i)
    }
}

func TestFixedWindow_Exceeded(t *testing.T) {
    limiter := NewFixedWindow(5, time.Second)
    for i := 0; i < 5; i++ {
        limiter.Allow("user-1")
    }
    assert.False(t, limiter.Allow("user-1"), "6th request should be denied")
}

func TestSlidingWindow_PreciseRecentCount(t *testing.T) {
    limiter := NewSlidingWindow(5, time.Second)

    // 提前 1.1 秒发 5 次（已过期）
    old := time.Now().Add(-1100 * time.Millisecond)
    for i := 0; i < 5; i++ {
        limiter.AllowAt("user-1", old)
    }

    // 现在再发 5 次 → 全部放行（旧的已滑出窗口）
    for i := 0; i < 5; i++ {
        assert.True(t, limiter.Allow("user-1"))
    }
}

func TestTokenBucket_TokensRefillOverTime(t *testing.T) {
    bucket := NewTokenBucket(10, time.Second) // 10 token/s
    // 消耗所有 token
    for i := 0; i < 10; i++ {
        assert.True(t, bucket.Allow("key"))
    }
    assert.False(t, bucket.Allow("key"))

    // 等 200ms → 恢复 2 个 token
    time.Sleep(200 * time.Millisecond)
    assert.True(t, bucket.Allow("key"))
    assert.True(t, bucket.Allow("key"))
    assert.False(t, bucket.Allow("key"))
}

func TestLeakyBucket_ConstantRate(t *testing.T) {
    bucket := NewLeakyBucket(2, time.Millisecond*100) // 2 次/100ms

    // 队列容量 5，前 5 个入队
    for i := 0; i < 5; i++ {
        assert.True(t, bucket.Allow("key"), "first 5 should be queued")
    }
    // 第 6 个溢出
    assert.False(t, bucket.Allow("key"), "6th should overflow")
}
```

### Day 2 验收

```bash
go test ./internal/limiter/ -v
# PASS: FixedWindow x4
# PASS: SlidingWindow x3
# PASS: TokenBucket x3
# PASS: LeakyBucket x2
# PASS: 集成中间件 x4
# 共 16 条 PASS
```

---

## Day 3 测试计划：熔断器 + 动态配置

**对应开发：** Day 3 — CircuitBreaker、fsnotify 热重载

### 熔断器（8 条）

| # | 测试内容 | 层级 |
|---|---|---|
| T3.1 | 初始状态 Closed | 单元 Small |
| T3.2 | 正常请求 → 不增加错误计数 | 单元 Small |
| T3.3 | 错误请求 → 错误计数 +1 | 单元 Small |
| T3.4 | 连续 5 次错误 → 状态变为 Open | 单元 Small |
| T3.5 | Open 状态下 → 请求直接拒绝（不调上游） | 单元 Small |
| T3.6 | 冷却时间到 → 状态变为 HalfOpen | 单元 Small |
| T3.7 | HalfOpen 试探成功 → 恢复 Closed | 单元 Small |
| T3.8 | HalfOpen 试探失败 → 回到 Open | 单元 Small |

### 动态配置（4 条）

| # | 测试内容 | 层级 |
|---|---|---|
| T3.9 | 修改 yaml → fsnotify 事件触发 | 集成 Medium |
| T3.10 | 新增路由 → 立即生效 | 集成 Medium |
| T3.11 | 删除路由 → 立即不可达 | 集成 Medium |
| T3.12 | Admin API POST `/admin/routes/reload` → 手动触发重载 | 集成 Medium |

### Day 3 核心用例

```go
func TestCircuitBreaker_StateTransitions(t *testing.T) {
    breaker := NewCircuitBreaker(CircuitBreakerConfig{
        Threshold:    3,
        CoolDown:     500 * time.Millisecond,
        HalfOpenMax:  1,
    })

    // Closed → 记录错误
    assert.Equal(t, StateClosed, breaker.State())
    breaker.RecordFailure() // 1
    breaker.RecordFailure() // 2
    breaker.RecordFailure() // 3 → Open!
    assert.Equal(t, StateOpen, breaker.State())

    // Open → 直接拒绝
    assert.False(t, breaker.Allow())

    // 冷却后 → HalfOpen
    time.Sleep(600 * time.Millisecond)
    assert.Equal(t, StateHalfOpen, breaker.State())
    assert.True(t, breaker.Allow()) // 试探通过

    // 试探成功 → Closed
    breaker.RecordSuccess()
    assert.Equal(t, StateClosed, breaker.State())
}

func TestCircuitBreaker_HalfOpenFailure_BackToOpen(t *testing.T) {
    breaker := NewCircuitBreaker(CircuitBreakerConfig{
        Threshold: 1, CoolDown: 100 * time.Millisecond,
    })
    breaker.RecordFailure() // → Open
    time.Sleep(150 * time.Millisecond) // → HalfOpen

    breaker.RecordFailure() // 试探失败
    assert.Equal(t, StateOpen, breaker.State())
}
```

### Day 3 验收

```bash
go test ./internal/breaker/ -v
# PASS: StateTransitions (7 个子场景)
# PASS: HalfOpenFailure → Open

go test ./internal/config/ -v -tags=integration
# PASS: HotReload
```

---

## Day 4 测试计划：插件架构 + Admin API

**对应开发：** Day 4 — Plugin 接口、Registry、Admin API

| # | 测试内容 | 层级 |
|---|---|---|
| T4.1 | Plugin 接口实现 | 单元 Small |
| T4.2 | Registry.Register：注册插件 | 单元 Small |
| T4.3 | Registry.GetChain：按优先级排序返回 | 单元 Small |
| T4.4 | Plugin 执行链：遍历所有插件 | 单元 Small |
| T4.5 | AuthPlugin 作为 Plugin 加载 | 集成 Medium |
| T4.6 | RateLimitPlugin 作为 Plugin 加载 | 集成 Medium |
| T4.7 | GET /admin/routes 返回当前路由表 | 集成 Medium |
| T4.8 | GET /admin/breakers 返回熔断器状态 | 集成 Medium |

### Day 4 核心用例

```go
func TestPluginRegistry_OrderedByPriority(t *testing.T) {
    reg := NewRegistry()
    reg.Register(&FakePlugin{name: "auth", priority: 100})
    reg.Register(&FakePlugin{name: "logger", priority: 50})
    reg.Register(&FakePlugin{name: "rateLimit", priority: 200})

    chain := reg.GetChain()
    assert.Len(t, chain, 3)
    assert.Equal(t, "rateLimit", chain[0].Name()) // priority 200 first
    assert.Equal(t, "auth", chain[1].Name())       // priority 100
    assert.Equal(t, "logger", chain[2].Name())     // priority 50
}

func TestPlugin_ThrowsIfChainFails(t *testing.T) {
    // 一个插件返回错误 → 整个链中断
    reg := NewRegistry()
    reg.Register(&ErrorPlugin{})
    reg.Register(&FakePlugin{name: "never_reached"})

    chain := reg.GetChain()
    c := createTestContext()
    chain.Execute(c)

    assert.True(t, c.IsAborted()) // 请求被中断
}
```

### Day 4 验收

```bash
go test ./internal/plugin/ -v
# PASS: Register/GetChain/Ordered
# PASS: Chain fails on plugin error
```

---

## Day 5 测试计划：E2E 全链路

**对应开发：** Day 5 — Docker 集成 + 文档

| # | 测试内容 | 层级 |
|---|---|---|
| T5.1 | `/blog/ping` → 转发 bloger → pong | E2E Large |
| T5.2 | `/seckill/ping` → 转发 seckill → pong | E2E Large |
| T5.3 | 无 token → 401 | E2E Large |
| T5.4 | 有效 token → 放行 + X-User-Id | E2E Large |
| T5.5 | 超频请求 → 429 | E2E Large |
| T5.6 | 4 种限流算法对比 E2E | E2E Large |

### Day 5 E2E 脚本

```bash
#!/bin/bash
# 1. Gateway 转发
check "blog ping" "pong" "$(curl -s $GW/blog/api/v1/ping)"
check "seckill ping" "pong" "$(curl -s $GW/seckill/api/v1/ping)"

# 2. 无 token → 401
check "no token 401" "401" "$(curl -s -o /dev/null -w '%{http_code}' $GW/blog/api/v1/users/me)"

# 3. 有效 token → 200
check "with token 200" "200" "$(curl -s -o /dev/null -w '%{http_code}' -H 'Authorization: Bearer $TOKEN' $GW/blog/api/v1/users/me)"

# 4. 限流超频 → 429
for i in $(seq 1 7); do
    code=$(curl -s -o /dev/null -w '%{http_code}' $GW/seckill/api/v1/seckill/buy)
    if [ "$code" = "429" ]; then
        echo "  ✅ rate limit 429 at request $i"
        break
    fi
done
```

---

## 测试统计

| 天 | 单元(Small) | 集成(Medium) | E2E(Large) | 累计 |
|---|---|---|---|---|
| Day 1 | 4 | 4 | 0 | 8 |
| Day 2 | 12 | 4 | 0 | 24 |
| Day 3 | 8 | 4 | 0 | 36 |
| Day 4 | 4 | 4 | 0 | 44 |
| Day 5 | 0 | 0 | 6 | **50** |

**金字塔比例：** 单元 28 (56%) / 集成 16 (32%) / E2E 6 (12%)

> 网关的集成测试比例高于前三个项目，因为反向代理必须通过 httptest 验证转发行为。

---

## 测试工具

| 工具 | 用途 |
|---|---|
| `testify/assert` | 断言 |
| `httptest.NewServer` | **模拟上游服务**（网关测试核心工具） |
| `httptest.NewRecorder` | HTTP 响应录制 |
| `net/http/httputil.ReverseProxy` | 真实的 Go 标准库代理 |
| 真实 JWT | 认证集成测试 |
| Docker Compose | Day 5 E2E 多上游 |

---

## 与前三个项目的测试差异

| 维度 | IM/博客/秒杀 | API 网关 |
|---|---|---|
| 核心测试模式 | 业务逻辑验证 | **代理转发 + 上游模拟** |
| 新增工具 | — | **httptest.NewServer 模拟上游** |
| 最难测试 | 并发竞态 | **熔断器状态机全路径** |
| 最有趣的测试 | — | **4 种限流算法对比验证** |
