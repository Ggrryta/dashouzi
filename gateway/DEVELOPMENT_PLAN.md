# API 网关 - 5 天开发排期计划

> 每天 4-6 小时。垂直切片：每天交付一个可验证的功能模块。
> 网关是基础设施型项目，和前三个业务项目不同——它的上游服务是其他项目。

---

## Day 1：骨架 + 反向代理 + 路由转发

**目标：** 网关接收请求，根据 Path 转发到上游服务，返回结果。

### 上午（3h）

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 1.1 | `go mod init gateway`，Gin 空壳 | `cmd/gateway/main.go` | `/health` 返回 OK |
| 1.2 | 复用 pkg/（errcode, response, logger） | `pkg/` | 编译通过 |
| 1.3 | 路由配置结构体 + yaml 加载 | `internal/config/` | 解析 routes 数组 |
| 1.4 | 路由表：Path → Upstream 映射 | `internal/router/table.go` | Add/Get/Match 可用 |

### 下午（3h）

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 1.5 | `httputil.ReverseProxy` 转发 | `internal/proxy/proxy.go` | 请求网关 → 转发上游 → 返回 |
| 1.6 | Gin 中间件：根据 Path 查路由表 → 转发 | `internal/middleware/proxy.go` | 一步完成路由+转发 |
| 1.7 | 路径前缀剥离（`/blog/api` → `api`） | proxy 内 | 上游收到正确路径 |
| 1.8 | 上游健康检查 | `internal/proxy/health.go` | 定时 GET 上游 /health |
| 1.9 | Dockerfile + docker-compose（gateway + bloger） | 容器编排 | `curl gateway/blog/ping → bloger pong` |

### Day 1 验收

```bash
# 配置
routes:
  - path: "/blog"
    upstream: "http://bloger:8080"
    strip_prefix: true

curl http://localhost:9000/blog/api/v1/ping
# → 转发到 bloger:8080 → bloger 的 pong 响应
```

---

## Day 2：JWT 认证 + 限流算法

**目标：** 网关统一做 JWT 认证，实现四种限流算法。

### 上午（3h）：JWT 认证

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 2.1 | JWT 公共校验中间件 | `internal/middleware/auth.go` | 无 token→401，有效→放行 |
| 2.2 | 将 user_id 注入 Header 传给上游 | auth 中间件 | 上游收到 `X-User-Id` |
| 2.3 | 路由级白名单（哪些 path 免认证） | 配置 + 中间件 | `/health` 免认证 |

### 下午（3h）：限流算法

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 2.4 | Limiter 接口定义 | `internal/limiter/limiter.go` | Allow(key) bool |
| 2.5 | 固定窗口计数器 | `internal/limiter/fixed_window.go` | 窗口内计数，超限拒绝 |
| 2.6 | **滑动窗口**（时间戳队列） | `internal/limiter/sliding_window.go` | 精确计数最近 N 秒 |
| 2.7 | 令牌桶 | `internal/limiter/token_bucket.go` | 固定速率放令牌，支持突发 |
| 2.8 | 漏桶 | `internal/limiter/leaky_bucket.go` | 固定速率处理，流量整形 |
| 2.9 | 路由级限流配置 + 测试 | 单元测试 | 四种算法各自验证 |

### Day 2 验收

```bash
# 带 token 访问 → 放行
curl -H "Authorization: Bearer <jwt>" http://localhost:9000/blog/api/v1/users/me
# 无 token → 401

# 限流：1秒内 6 次请求 → 前5次 200，第6次 429
```

---

## Day 3：熔断器 + 动态配置

**目标：** 上游故障自动熔断，路由规则热更新。

### 上午（3h）：熔断器

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 3.1 | CircuitBreaker 状态机 | `internal/breaker/breaker.go` | Closed/Open/HalfOpen |
| 3.2 | 错误计数 + 阈值判断 | breaker 内 | 5 次连续错误 → Open |
| 3.3 | 冷却时间 + HalfOpen 试探 | breaker 内 | 30s 后试探 1 次 |
| 3.4 | 集成到 Proxy 中间件 | proxy 内 | 上游错误时走熔断逻辑 |
| 3.5 | 熔断器状态查询接口 | Admin API | GET /admin/breakers |

### 下午（3h）：动态配置

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 3.6 | Viper WatchConfig + fsnotify | 启动 goroutine | 文件变更回调触发 |
| 3.7 | 路由表热重载 | `internal/router/table.go` | 原子替换，不丢请求 |
| 3.8 | 重载接口 | POST `/admin/routes/reload` | curl 触发重载 |
| 3.9 | 测试：修改 yaml → 路由生效 | E2E | 新增路由后立即生效 |

### Day 3 验收

```bash
# 模拟上游挂掉（docker stop bloger）
curl /blog/api/v1/ping
# 第1-5次 → 502 Bad Gateway
# 第6次 → 503 Circuit Breaker Open

# 30s 后 → 503（保持 Open）
# 重启 bloger → 等待 HalfOpen 试探 → 恢复

# 热更新：修改 routes.yaml，POST /admin/routes/reload
curl /new-service/api/ping
# → 新路由立即生效
```

---

## Day 4：插件架构 + Admin API

**目标：** 中间件以插件形式加载，完善管理接口。

### 上午（3h）：插件架构

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 4.1 | Plugin 接口定义 | `internal/plugin/plugin.go` | Name / Priority / Handle |
| 4.2 | 插件注册器 | `internal/plugin/registry.go` | 按优先级排序执行 |
| 4.3 | RateLimit 插件化 | 重构限流为 plugin | 通过注册器加载 |
| 4.4 | Auth 插件化 | 重构认证为 plugin | 通过注册器加载 |
| 4.5 | 插件执行链 | `internal/plugin/chain.go` | 遍历注册器执行 |

### 下午（3h）：Admin API

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 4.6 | Admin 路由组 | `/admin/*` | 管理接口独立端口 |
| 4.7 | 路由表查询 + 重载接口 | 完善 | GET / POST |
| 4.8 | 熔断器状态接口 | 完善 | GET /admin/breakers |
| 4.9 | 限流统计接口 | GET `/admin/limiters` | 各路由限流状态 |

### Day 4 验收

```bash
# 插件化后：认证、限流、熔断都是插件
# 通过配置文件决定加载哪些插件

# Admin API
curl /admin/routes          # 当前路由表
curl /admin/breakers        # 熔断器状态
curl /admin/routes/reload   # 热重载路由
```

---

## Day 5：Docker 集成 + E2E + 文档

**目标：** Docker Compose 编排 gateway + 三个上游服务，全链路验证。

### 上午（3h）

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 5.1 | 多上游 Docker Compose | gateway + bloger + seckill_gateway | 4 容器全绿 |
| 5.2 | 多路由规则配置 | routes.yaml | 3 条路由 |
| 5.3 | 全链路转发 | E2E | `/blog/ping` `/seckill/ping` 都能通 |

### 下午（3h）

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 5.4 | E2E 测试脚本 | `scripts/e2e.sh` | 路由/认证/限流/熔断 全覆盖 |
| 5.5 | README.md | 项目根目录 | 架构图 + API 文档 |
| 5.6 | 4 种限流算法对比测试 | 压测脚本 | QPS + 拒绝率数据 |
| 5.7 | GitHub push | 公开仓库 | — |

### Day 5 验收

```bash
./scripts/e2e.sh
# ✅ 路由转发：/blog → bloger, /seckill → seckill
# ✅ JWT 认证：无 token 拒绝 / 有效 token 放行
# ✅ 限流：超频返回 429
# ✅ 熔断：上游挂掉 → 断路器 Open → 恢复
# ✅ 热重载：修改配置 → 新路由生效
```

---

## 整体时间线

```
Day1        Day2        Day3         Day4         Day5
代理+路由    认证+限流    熔断+热配    插件+Admin   集成+E2E
  ██          ██          ██          ██           ██
  └────── docker compose 从 Day1 跑到 Day5 ─────────┘
```

---

## 与前三个项目的差异

| 维度 | 博客/秒杀/IM | API 网关 |
|---|---|---|
| 项目角色 | 业务服务 | **通用基础设施** |
| 核心代码 | gin + gorm + websocket | **httputil.ReverseProxy** |
| 新能力 | — | **4种限流算法、熔断器、插件化** |
| 上游依赖 | 外部（MySQL/Redis） | **其他项目作为上游** |
| 可复用性 | 业务特定 | **可对接任意 HTTP 服务** |

---

## 验收检查清单

- [ ] 路由转发：Path 匹配 → 上游返回正确结果
- [ ] 路径前缀剥离：上游收到去掉前缀的路径
- [ ] JWT 认证：无 token→401 / 有效→放行 + Header 注入
- [ ] 4 种限流算法全部实现，各自可独立验证
- [ ] 限流接口级配置（不同 path 不同策略）
- [ ] 熔断器状态机：Closed → Open → HalfOpen → Closed
- [ ] 路由热更新：修改 yaml 后不重启即生效
- [ ] 插件化：认证/限流/熔断以 Plugin 形式加载
- [ ] Admin API 可查询路由表、熔断器状态
- [ ] 全链路 E2E：gateway → 多个上游服务
