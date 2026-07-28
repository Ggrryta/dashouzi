# API Gateway

Go 实现的 API 网关，支持路由转发、JWT 认证、4 种限流算法、熔断器、插件化架构。

## 技术栈

| 模块 | 选型 |
|---|---|
| 代理 | `net/http/httputil.ReverseProxy` |
| 限流 | 固定窗口 / 滑动窗口 / 令牌桶 / 漏桶 |
| 熔断 | Closed → Open → HalfOpen 状态机 |
| 配置 | yaml + fsnotify 热更新 |
| 插件 | interface + Registry 按优先级执行 |

## 快速启动

```bash
go run ./cmd/gateway
curl http://localhost:9000/health
curl http://localhost:9000/blog/api/v1/ping  # → bloger pong
```

## 路由配置

```yaml
routes:
  - path: "/blog"
    upstream: "http://localhost:8080"
  - path: "/seckill"
    upstream: "http://localhost:8081"
```

## 架构

```
Request → [Auth Plugin] → [RateLimit Plugin] → Route Match → ReverseProxy → Upstream
```

## 测试

```bash
go test ./... -v
```
