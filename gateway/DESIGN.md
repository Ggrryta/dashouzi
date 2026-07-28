# API 网关 - 架构设计文档

## 1. 项目定位

博客(Level 1)、秒杀(Level 2)、IM(Level 3) 都是业务型项目，网关(Level 4) 是**基础设施型项目**。

它不是给用户用的，是给服务端开发者用的——统一所有微服务的入口，做认证、限流、路由、日志。

## 2. 技术栈

| 层 | 选型 | 为什么 |
|---|---|---|
| 语言 | Go 1.23+ | 高性能网络编程 |
| HTTP 代理 | `net/http/httputil.ReverseProxy` | Go 标准库，零拷贝 |
| 路由 | Gin | 统一技术栈 |
| 配置 | Viper + fsnotify | 热更新路由规则 |
| 限流 | 自实现滑动窗口 | 面试必考算法 |
| 熔断 | 自实现断路器 | 理解原理而非用库 |
| 认证 | JWT 公共校验 | 统一入口认证 |
| 监控 | OpenTelemetry | 链路追踪 |

## 3. 整体架构

```
                         ┌─────────────────┐
    用户请求 ──────────→  │   API Gateway    │
                         │                 │
                         │ 1. 限流          │  请求先过中间件链
                         │ 2. JWT 认证      │
                         │ 3. 路由匹配      │  Path → 上游服务
                         │ 4. 负载均衡      │
                         │ 5. 转发          │  httputil.ReverseProxy
                         │ 6. 熔断          │  上游故障自动切断
                         │ 7. 日志/追踪     │
                         └────────┬────────┘
                                  │
                    ┌─────────────┼─────────────┐
                    ▼             ▼             ▼
              ┌─────────┐  ┌─────────┐  ┌─────────┐
              │ bloger  │  │ seckill │  │   im    │
              │ :8080   │  │ :8081   │  │ :8082   │
              └─────────┘  └─────────┘  └─────────┘
```

## 4. 核心功能

### 4.1 路由转发

根据 URL Path 转发到不同上游服务：

```yaml
routes:
  - path: "/blog/*"
    upstream: "http://localhost:8080"
  - path: "/seckill/*"
    upstream: "http://localhost:8081"
  - path: "/im/*"
    upstream: "http://localhost:8082"
```

实现：`httputil.ReverseProxy`，支持路径前缀剥离。

### 4.2 认证鉴权

统一在网关层校验 JWT，后端服务不再各自认证：

```go
// 网关中间件
func AuthMiddleware(jwt *JWT) gin.HandlerFunc {
    // 解析 Authorization: Bearer <token>
    // 校验 → 放行或拒绝
    // 将 user_id 写入 Header 传给上游
}
```

### 4.3 限流 — 滑动窗口

实现四种限流算法，可按接口配置：

| 算法 | 原理 | 适用场景 |
|---|---|---|
| 固定窗口 | 每个时间窗口计数 | 简单场景 |
| 滑动窗口 | 记录每次请求时间戳 | 平滑限流 |
| 令牌桶 | 固定速率放令牌 | 允许突发 |
| 漏桶 | 固定速率处理 | 流量整形 |

### 4.4 熔断器

上游服务异常时自动熔断，保护系统：

```
状态机: Closed → (错误数达标) → Open → (冷却时间到) → HalfOpen → (试探成功) → Closed
                                                          └→ (试探失败) → Open
```

```go
type CircuitBreaker struct {
    state         State       // Closed / Open / HalfOpen
    failureCount  int         // 当前失败数
    threshold     int         // 熔断阈值
    coolDown      time.Duration // 冷却时间
    lastFailure   time.Time
}
```

### 4.5 动态配置

路由规则支持热更新，不重启网关：

```go
// 监听配置文件变化
viper.WatchConfig()
viper.OnConfigChange(func(e fsnotify.Event) {
    reloadRoutes()
})
```

### 4.6 插件化架构

认证、限流、日志、熔断以插件形式加载：

```go
type Plugin interface {
    Name() string
    Priority() int        // 执行顺序
    Handle(c *gin.Context) // 中间件函数
}
```

## 5. 中间件链执行顺序

```
请求 → RateLimit → Auth → Route → CircuitBreaker → ReverseProxy → Logger → 响应
```

## 6. 接口设计

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/admin/routes` | 查看当前路由表 |
| POST | `/admin/routes/reload` | 热加载路由配置 |
| GET | `/admin/breakers` | 查看熔断器状态 |
| POST | `/admin/breakers/reset` | 手动重置熔断器 |
| GET | `/health` | 网关健康检查 |

## 7. 项目结构

```
gateway/
├── cmd/gateway/main.go
├── internal/
│   ├── config/        # 配置 + 热更新
│   ├── proxy/         # 反向代理
│   ├── router/        # 路由表管理
│   ├── limiter/       # 限流算法
│   ├── breaker/       # 熔断器
│   ├── middleware/     # 认证中间件
│   ├── plugin/        # 插件接口 + 注册
│   └── admin/         # 管理接口
├── pkg/               # 公共包
├── config/gateway.yaml
├── docker-compose.yml # gateway + 3 个上游服务
└── Dockerfile
```

## 8. Docker 编排

```yaml
services:
  gateway:      # API 网关
  bloger:       # 上游：博客系统
  seckill:      # 上游：秒杀系统（仅路由，无完整功能）
  im-gateway:   # 上游：IM 网关
```

## 9. 对比已学项目

| | 博客/秒杀/IM | API 网关 |
|---|---|---|
| 角色 | 业务服务 | **基础设施** |
| 流量方向 | 接收请求 | **接收 + 转发** |
| 新概念 | — | **反向代理、熔断、动态配置** |
| 标准库 | gin/gorm | **httputil.ReverseProxy** |
| 插件化 | 无 | **Plugin 接口** |

## 10. 面试高频问题

| 问题 | 答案要点 |
|---|---|
| 网关和负载均衡有什么区别？ | 网关在七层(HTTP)，LB 在四层(TCP) |
| 你实现了哪几种限流算法？ | 固定窗口、滑动窗口、令牌桶、漏桶 |
| 熔断器三种状态是怎么切换的？ | Closed→Open→HalfOpen→Closed |
| 怎么实现路由热更新？ | fsnotify 监听文件 + 原子替换路由表 |
