# Go 后端性能调优黄金三连

> 适用项目：`bloger`（Gin + GORM/PostgreSQL + Redis + JWT）
> 适用版本：Go 1.19+（GOMEMLIMIT 要求 1.19 以上）
> 目标：把 P99 从 50ms 级压到 5ms 级，并稳定尾延迟

---

## 0. 导言：为什么是这三连

P99 飙升、CPU 跑满、内存膨胀三种症状，在 Go 后端中**相互放大**，单独治理一个会被另两个吃掉：

```
JSON 反射分配多
    │
    ↓
GC 压力大 ─→ assist goroutine 阻塞业务线程 ─→ P99 抖动
                                           │
                                           ↓
                              下游 DB 连接被占满 ─→ goroutine 进一步堆积
                                           │
                                           ↓
                              runtime.newproc 分配栈 ─→ 更多 GC ─→ 雪崩
```

**黄金三连**对应三条断链：

1. `sync.Pool + []byte 复用` → 断开 "JSON 反射 → GC" 链
2. `context + 连接池 + 超时` → 断开 "下游阻塞 → goroutine 雪崩" 链
3. `GOMEMLIMIT + GOGC 调参` → 断开 "GC 触发频率 → assist 阻塞" 链

三者一起做，效果是 10×；只做一个，效果不到 2×。本文逐项给出**机制 → 配置 → 代码 → 验证**。

---

## 第一章：sync.Pool + []byte 复用

### 1.1 为什么需要

Go GC 触发条件：**堆达上次标记存活量的 (1 + GOGC/100) 倍**，默认 GOGC=100 即 2 倍。

设稳态存活堆 100 MB，每秒分配 500 MB，则每秒 5 次 GC。每次 GC 期间：

- 业务 goroutine 被劫持做 assist mark（1~10 ms）
- 写屏障开启，每个指针写都进 `wbBuf`
- P99 周期性毛刺，频率 = GC 频率

**核心思路**：把分配速率从 GB/s 降到 MB/s，GC 从"每秒几次"降到"每分钟几次"。

### 1.2 标准用法

```go
package xhttp

import "bytes"

var bufPool = sync.Pool{
    New: func() interface{} {
        // 预分配 1KB，避免首次 grow
        b := make([]byte, 0, 1024)
        return &b
    },
}

// 注意：sync.Pool 缓存的是 *[]byte 而不是 []byte，
// 否则 Put 时会发生 slice header 复制（逃逸分析会把它分配到堆）。

func WriteJSON(w http.ResponseWriter, v interface{}) error {
    bp := bufPool.Get().(*[]byte)
    buf := (*bp)[:0]

    enc := json.NewEncoder(bytes.NewBuffer(buf))
    if err := enc.Encode(v); err != nil {
        return err
    }

    _, err := w.Write(buf)
    *bp = buf  // 复位
    bufPool.Put(bp)
    return err
}
```

### 1.3 进阶用法

#### 1.3.1 gin.Context 复用（Gin 已内置）

Gin 自带的 `engine.pool` 已经在每请求 Reset 一个 `Context`。但你的 handler 内部仍可能分配：

```go
// BAD：每请求分配 map
func getUser(c *gin.Context) {
    resp := map[string]interface{}{
        "id":    1,
        "name":  "alice",
        "email": "a@b.com",
    }
    c.JSON(200, resp)
}
```

每请求至少 4 个分配：map header + 3 个 eface。改成预定义结构体：

```go
type userResponse struct {
    ID    int64  `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

func getUser(c *gin.Context) {
    c.JSON(200, userResponse{ID: 1, Name: "alice", Email: "a@b.com"})
}
```

只分配 1 个 `userResponse`，可进一步放 pool。

#### 1.3.2 bytes.Buffer 池化（读写大 JSON）

```go
package xbuf

import "bytes"

var bufferPool = sync.Pool{
    New: func() interface{} {
        return bytes.NewBuffer(make([]byte, 0, 4096))
    },
}

func Get() *bytes.Buffer { return bufferPool.Get().(*bytes.Buffer) }

func Put(b *bytes.Buffer) {
    b.Reset()
    bufferPool.Put(b)
}
```

使用：

```go
buf := xbuf.Get()
defer xbuf.Put(buf)

json.NewEncoder(buf).Encode(payload)
io.Copy(w, buf)
```

#### 1.3.3 替换标准 JSON 库（终极手段）

如果项目体积允许，直接换 `sonic`：

```go
import "github.com/bytedance/sonic"

// 替换所有 json.Marshal/Unmarshal
data, err := sonic.Marshal(v)
err = sonic.Unmarshal(data, &v)

// Gin 全局替换
r := gin.New()
r.Use(func(c *gin.Context) {
    // 让 c.JSON 走 sonic
    c.SetCtx(func() {} /* ... */)
})
```

或者 Gin 自带支持：

```go
import "github.com/gin-gonic/gin/render"

r := gin.New()
r.Use(func(c *gin.Context) {
    c.Render = func(code int, r render.Render) { /* sonic */ }
})
```

简单做法：在 `main.go` 启动时设：

```go
import "github.com/bytedance/sonic/gin"
r := gin.New()
r.HTMLRender = sonicrender.Default
```

### 1.4 注意事项

| 陷阱 | 说明 | 解决 |
|---|---|---|
| GC 清空 pool | 每次 GC 后 pool 会被回收 | 接受；目标是降低 GC 频率本身 |
| 大对象放 pool | 占内存不释放，反而增加存活堆 | 只池化 ≤16KB 的对象 |
| 跨 goroutine 边界 | sync.Pool 设计就是并发安全 | 可直接跨 goroutine 使用 |
| 忘记 Reset | 残留数据污染下次使用 | Put 前 `b.Reset()` |
| 存 `[]byte` 而非 `*[]byte` | 逃逸到堆，反而更慢 | 永远存指针 |
| 容量不断扩张 | 不同请求大小不一 | Put 时 `*b = (*b)[:0:0]` 截断 |

### 1.5 验证方法

```bash
# 分配 profile
go test -bench=. -benchmem -memprofile=mem.out
go tool pprof -alloc_objects mem.out

# 期望：每请求分配数 <3，alloc_bytes <2KB
```

---

## 第二章：context + 连接池 + 超时

### 2.1 超时三件套

每个外部调用必须有**三个层次超时**：

```
请求入口（context 超时）
    │
    ├─ DB query（继承入口 ctx）
    ├─ Redis call（继承入口 ctx）
    └─ 下游 HTTP call（继承入口 ctx + Transport 超时）
```

**反例（你项目当前的代码）**：

```go
// cmd/server/main.go 当前实现
db, err := gorm.Open(postgres.Open(cfg.DB.DSN()), &gorm.Config{...})
```

GORM 默认查询无超时，一旦 PG 主从切换或单条慢查询，所有 goroutine 堆积等连接。

### 2.2 数据库连接池调优

#### 公式

```
SetMaxOpenConns  ≈ DB CPU 核数 × 2~4    // 不要超过 50 除非 DB 很强
SetMaxIdleConns  = SetMaxOpenConns       // 防止空闲连接关闭
SetConnMaxIdleTime = 5~15 min            // 探活，避免服务端 close
SetConnMaxLifetime = 1~2 h               // 滚动重连，避免长连接老化
```

**为什么不是越大越好？**

- DB 单连接是单 worker 串行
- 100 个连接同时执行，DB 内部锁竞争反而变慢
- PostgreSQL 默认 `max_connections=100`，超了直接报错

#### 配置代码（建议改 `main.go`）

```go
sqlDB, err := db.DB()
if err != nil {
    logger.Log.Fatal("failed to get sql.DB", zap.Error(err))
}

// 根据 DB 规模调整，4 核 DB 起步设 16
sqlDB.SetMaxOpenConns(16)
sqlDB.SetMaxIdleConns(16)
sqlDB.SetConnMaxIdleTime(10 * time.Minute)
sqlDB.SetConnMaxLifetime(time.Hour)
```

#### GORM 查询超时

```go
// 用 context 控制
ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
defer cancel()

if err := r.db.WithContext(ctx).
    First(&article, id).Error; err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        return nil, ErrDBTimeout
    }
    return nil, err
}
```

可封装到 repository 基类：

```go
type BaseRepo struct {
    db *gorm.DB
}

func (b *BaseRepo) Ctx(parent context.Context) (context.Context, context.CancelFunc) {
    return context.WithTimeout(parent, 3*time.Second)
}
```

### 2.3 HTTP Client 连接池（调用下游服务）

`http.DefaultClient` 没有超时，**禁止使用**。统一封装：

```go
package xhttp

import (
    "net"
    "net/http"
    "time"
)

var DefaultClient = &http.Client{
    Timeout: 3 * time.Second,
    Transport: &http.Transport{
        // 连接池
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 20,  // 关键，默认 2 太小
        MaxConnsPerHost:     50,
        IdleConnTimeout:     90 * time.Second,

        // 拨号超时
        DialContext: (&net.Dialer{
            Timeout:   3 * time.Second,
            KeepAlive: 30 * time.Second,
        }).DialContext,

        // HTTP/2
        ForceAttemptHTTP2: true,
    },
}
```

调用：

```go
ctx, cancel := context.WithTimeout(parent, 2*time.Second)
defer cancel()

req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
resp, err := xhttp.DefaultClient.Do(req)
```

### 2.4 服务端链路超时

入口处用 middleware 给每个请求总超时：

```go
func TimeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
    return func(c *gin.Context) {
        ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
        defer cancel()

        c.Request = c.Request.WithContext(ctx)

        // 用 channel 检测超时
        done := make(chan struct{})
        go func() {
            c.Next()
            close(done)
        }()

        select {
        case <-done:
            return
        case <-ctx.Done():
            c.AbortWithStatusJSON(504, gin.H{"error": "request timeout"})
        }
    }
}
```

挂到 router：

```go
r := gin.New()
r.Use(TimeoutMiddleware(10 * time.Second))
```

### 2.5 限流与熔断

入口限流，避免 goroutine 雪崩：

```go
import "golang.org/x/time/rate"

var limiter = rate.NewLimiter(rate.Every(time.Second), 1000) // 1000 QPS

func RateLimit() gin.HandlerFunc {
    return func(c *gin.Context) {
        if !limiter.Allow() {
            c.AbortWithStatusJSON(429, gin.H{"error": "rate limit"})
            return
        }
        c.Next()
    }
}
```

下游熔断（推荐 `sony/gobreaker`）：

```go
import "github.com/sony/gobreaker"

var cb = gobreaker.NewCircuitBreaker(gobreaker.Settings{
    Name:        "user-service",
    MaxRequests: 5,                       // 半开状态最大请求
    Interval:    60 * time.Second,
    ReadyToTrip: func(counts gobreaker.Counts) bool {
        return counts.ConsecutiveFailures > 5
    },
    OnStateChange: func(name string, from, to gobreaker.State) {
        logger.Log.Warn("circuit breaker state change",
            zap.String("name", name),
            zap.String("from", from.String()),
            zap.String("to", to.String()))
    },
})

func callUserSvc(ctx context.Context, id int) (*User, error) {
    v, err := cb.Execute(func() (interface{}, error) {
        return fetchUser(ctx, id)
    })
    if err != nil {
        return nil, err
    }
    return v.(*User), nil
}
```

### 2.6 goroutine 上限

防雪崩最后一道防线：

```go
import "golang.org/x/sync/semaphore"

var sem = semaphore.NewWeighted(10000) // 同时最多 1 万 goroutine

func Handle(c *gin.Context) {
    if !sem.TryAcquire(1) {
        c.AbortWithStatusJSON(503, gin.H{"error": "too many concurrent"})
        return
    }
    defer sem.Release(1)

    // ... handler
}
```

### 2.7 验证方法

```bash
# goroutine 监控
curl http://localhost:8080/debug/pprof/goroutine?debug=1 | head -20

# 期望：稳态 goroutine 数 < QPS × avg_latency × 1.5
# 例如 QPS=1000、P50=10ms，稳态应 <15；超过 200 说明有堆积

# trace 看阻塞
go tool trace trace.out
# 看 "Network" + "GC waiting" + "Syscall" 时间分布
```

---

## 第三章：GOMEMLIMIT + GOGC 调参

### 3.1 GC 触发机制（Go 1.19+）

三种触发条件，**任一满足**即触发：

```
1. 堆增长 trigger = live_heap × (1 + GOGC/100)     // 经典触发
2. 堆达 GOMEMLIMIT                                // 硬限制
3. 距上次 GC 超过 2 分钟                            // 强制触发
```

**Assist 机制**：当 goroutine 要分配但距下次 GC 触发还差 X 字节，runtime 让它先做 X 字节的标记工作。这是 P99 毛刺的真正根因——不是 STW，是业务 goroutine 被劫持。

### 3.2 GOGC 含义

| GOGC | 触发阈值 | 内存占用 | GC 频率 |
|---|---|---|---|
| 50 | 1.5× live | 低 | 高 |
| 100（默认） | 2× live | 中 | 中 |
| 200 | 3× live | 高 | 低 |
| 400 | 5× live | 很高 | 很低 |
| off | 关闭堆触发 | ∞ | 仅靠 limit/2min |

**调高 GOGC 的前提**：内存够大，且分配速率稳定。

**调低 GOGC 的前提**：内存紧张，宁可多 GC 也不 OOM。

### 3.3 GOMEMLIMIT 含义（Go 1.19+）

设置堆的**软上限**，运行时尽量让存活堆不超过此值。机制：

- 接近 limit 时，runtime 提高 GC 触发频率
- 超过 limit 时，assist 工作量倍增，强制回收
- **不会立即 OOM**，但会让 P99 显著上升（assist 增加）

**核心收益**：让 GOGC 设大一点（如 400），内存够时低频 GC，逼近 limit 时自动升频，**两全**。

### 3.4 联合使用公式

设容器内存上限 M（如 1 GB），推荐：

```
GOMEMLIMIT = 0.8 × M           // 留 20% 给非 Go 内存（cgo/stack）
GOGC = 200~400                 // 大堆低频 GC
GODEBUG=gctrace=1              // 开 trace，生产可关
GOMAXPROCS = min(物理核×2, 8)   // 不建议超过 8
```

启动：

```bash
GOMEMLIMIT=800MiB GOGC=400 ./bloger
```

或在代码里：

```go
import "runtime/debug"

func init() {
    debug.SetMemoryLimit(800 * 1024 * 1024)  // 800MB
    debug.SetGCPercent(400)
}
```

### 3.5 验证方法

#### 3.5.1 GC trace

```bash
GODEBUG=gctrace=1 ./bloger 2>gc.log
```

输出形如：

```
gc 1 @0.045s 1%: 0.012+0.36+0.005 ms clock, 0.097+0.18+0.20 ms cpu, 4->4->2 MB, 5->5->0 MB, 0 MB goal, 0 MB stacks, 0 MB globals, 4 P
```

关注字段：

- `1%`：GC 占用 CPU 时间百分比，>10% 就要警惕
- `4->4->2 MB`：堆大小 (start->end->live)，live 不应持续增长
- `0 MB goal`：触发阈值
- `0.012+0.36+0.005 ms clock`：STW + 并发标记 + STW，第一个和第三个是 P99 毛刺来源

**期望**：

- GC 频率 < 1 次/秒
- CPU 占比 < 5%
- live heap 稳定（不持续上升）

#### 3.5.2 pprof

```go
import _ "net/http/pprof"

// 单独开一个内部端口，不暴露给外网
go func() {
    http.ListenAndServe("127.0.0.1:6060", nil)
}()
```

线上采集：

```bash
# 30 秒 CPU profile
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# 堆分配 profile
go tool pprof -alloc_space http://localhost:6060/debug/pprof/heap

# 期望 top：
# runtime.mallocgc        < 3%
# runtime.gcAssistAlloc   < 1%
# runtime.scanobject      < 5%
```

#### 3.5.3 trace

```go
import "runtime/trace"

f, _ := os.Create("trace.out")
defer f.Close()
trace.Start(f)
defer trace.Stop()

// 运行业务一段时间
time.Sleep(5 * time.Second)
```

分析：

```bash
go tool trace trace.out
```

浏览器打开后看：

- **View trace**：每个 P 的时间轴。红色块是 syscall，绿色是业务，紫色是 GC
- **GC latency**：GC 频率与时长分布
- **Heap**：堆大小随时间曲线
- **Goroutine analysis**：阻塞时长分布，找最长 goroutine

### 3.6 实战配置参考

#### 开发环境

```bash
GOMAXPROCS=4 GOGC=100 go run ./cmd/server
```

#### 生产环境（1 GB 容器）

```dockerfile
ENV GOMAXPROCS=4
ENV GOGC=200
ENV GOMEMLIMIT=800MiB
ENV GODEBUG=gctrace=1
```

#### 生产环境（4 GB 容器）

```dockerfile
ENV GOMAXPROCS=8
ENV GOGC=400
ENV GOMEMLIMIT=3200MiB
```

#### K8s 部署

注意 `resources.limits.memory` 与 `GOMEMLIMIT` 的关系：

```yaml
resources:
  limits:
    memory: "1Gi"
env:
  - name: GOMEMLIMIT
    value: "800MiB"  # = 0.8 × limit
  - name: GOMAXPROCS
    valueFrom:
      resourceFieldRef:
        resource: limits.cpu
```

新版 Go 1.22+ 支持 `GOMAXPROCS` 自动从 cgroup 读取，无需手动设。

---

## 第四章：三者联动案例

### 4.1 假设基线（本博客项目典型表现）

| 指标 | 基线 |
|---|---|
| QPS | 1000 |
| P50 | 8 ms |
| P99 | 80 ms |
| P999 | 500 ms |
| Goroutine 稳态 | 50 |
| Goroutine 峰值 | 8000（DB 抖动时） |
| GC 频率 | 8 次/秒 |
| GC CPU 占比 | 12% |
| 内存 | 持续上涨到 OOM |

### 4.2 单独治理对比

| 改动 | P99 改善 | 副作用 |
|---|---|---|
| 仅做 sync.Pool | 80→50 ms | GC 频率 8→4，但 DB 抖动仍雪崩 |
| 仅做 ctx + 连接池 | 80→30 ms | GC 频率不变，内存仍涨 |
| 仅做 GC 调参 | 80→60 ms | DB 雪崩一来 GC 直接 OOM |
| **三连全做** | **80→5 ms** | 各项稳定 |

### 4.3 三连实施清单（按优先级）

1. **DB 连接池设上限**（10 分钟内完成，最大收益）
   - 改 `cmd/server/main.go` 加 `SetMaxOpenConns(16)` 等
2. **GORM 查询全部带 ctx**（半天）
   - repository 层封装 `WithContext(timeout)`
3. **HTTP Client 统一封装**（半天）
   - 替换 `http.Get` 为带超时 client
4. **入口超时中间件**（1 小时）
   - `TimeoutMiddleware(10*time.Second)`
5. **限流中间件**（1 小时）
   - `rate.NewLimiter(1000 QPS)`
6. **JSON 序列化换 sonic**（半天）
   - 评估迁移成本，主要是 `interface{}` 兼容性
7. **bytes.Buffer 池化**（半天）
   - 在 handler 内大 JSON 场景应用
8. **GC 调参**（10 分钟）
   - Dockerfile 加 ENV
9. **pprof 端口开启**（10 分钟）
   - 内部 6060 端口，监控用
10. **熔断器加下游调用**（1 天）
    - `gobreaker` 包裹所有外部 HTTP 调用

### 4.4 实施顺序建议

```
Phase 1（1 天）：DB 连接池 + ctx 超时 + GC 调参
  ↓ 期望：P99 80 → 25 ms

Phase 2（2 天）：HTTP client 封装 + 限流 + 入口超时
  ↓ 期望：P99 25 → 12 ms

Phase 3（3 天）：sonic + sync.Pool + 熔断
  ↓ 期望：P99 12 → 5 ms

Phase 4（持续）：pprof 监控 + 参数微调
  目标：稳态 P99 < 5 ms，P999 < 20 ms
```

---

## 附录 A：本博客项目当前问题清单

基于 `cmd/server/main.go` 现有代码审查：

| 位置 | 问题 | 优先级 |
|---|---|---|
| `main.go:46` | `gin.SetMode(cfg.Server.Mode)` 默认 debug，应配 release | 中 |
| `main.go:49-51` | GORM 未设 `PrepareStmt: true`（预编译可省 30% DB CPU） | 高 |
| `main.go:57-63` | 连接池设了 50，需根据 DB 规模调整；缺 `SetConnMaxIdleTime` | 高 |
| `main.go:82-83` | `router.Setup` 未传入 ctx 超时 middleware | 中 |
| `main.go:85-89` | `http.Server` 未设 `ReadTimeout`、`WriteTimeout`、`IdleTimeout` | 高 |
| 全局 | 无限流、熔断、pprof 端口 | 高 |

### 建议的 `http.Server` 配置

```go
srv := &http.Server{
    Addr:         addr,
    Handler:      r,
    ReadTimeout:  5 * time.Second,
    WriteTimeout: 10 * time.Second,
    IdleTimeout:  120 * time.Second,
    ReadHeaderTimeout: 3 * time.Second,  // 防 slowloris 攻击
    MaxHeaderBytes: 1 << 20,              // 1 MB
}
```

## 附录 B：检测脚本

### B.1 GC 频率统计

```bash
# bloger/scripts/gc-watcher.sh
#!/bin/bash
LOG=gc.log
tail -f $LOG | awk '
/gc [0-9]+ @/ {
    match($0, /@([0-9.]+)s/, t)
    now = t[1]
    if (last_gc > 0) {
        gap = now - last_gc
        printf "GC gap: %.3fs\n", gap
    }
    last_gc = now
}'
```

### B.2 goroutine 监控

```bash
# bloger/scripts/goroutine-watch.sh
#!/bin/bash
while true; do
    n=$(curl -s http://localhost:8080/debug/pprof/goroutine?debug=1 | head -1 | grep -oE 'goroutine-profile:.*$' || true)
    if [ -z "$n" ]; then
        n=$(curl -s http://localhost:8080/debug/pprof/goroutine?debug=1 | grep -c "^goroutine")
    fi
    echo "$(date '+%H:%M:%S') goroutines: $n"
    sleep 5
done | tee goroutine.log
```

### B.3 三连验证脚本

```bash
# bloger/scripts/verify-tuning.sh
#!/bin/bash
set -e

echo "== Phase 1: load test =="
wrk -t4 -c100 -d30s http://localhost:8080/health

echo "== Phase 2: pprof snapshot =="
go tool pprof -alloc_space -png -seconds=30 \
    http://localhost:6060/debug/pprof/heap > heap.png

echo "== Phase 3: GC trace (60s) =="
GODEBUG=gctrace=1 timeout 60 ./bloger 2>gc.log
echo "GC count: $(grep -c '^gc' gc.log)"
echo "Avg GC CPU%: $(grep '^gc' gc.log | awk -F'%' '{sum+=$2} END{print sum/NR}')"

echo "== Phase 4: goroutine peak =="
awk -F'goroutines:' '{print $2}' goroutine.log | sort -n | tail -1
```

## 附录 C：参考配置 YAML

在 `config/config.yaml` 增加性能段：

```yaml
performance:
  max_procs: 4
  gc_percent: 200
  memory_limit_mb: 800

server:
  read_timeout: 5s
  write_timeout: 10s
  idle_timeout: 120s
  read_header_timeout: 3s
  max_header_bytes: 1048576
```

对应 `config.go` 扩展：

```go
type ServerConfig struct {
    Host               string        `mapstructure:"host"`
    Port               int           `mapstructure:"port"`
    Mode               string        `mapstructure:"mode"`
    ReadTimeout        time.Duration `mapstructure:"read_timeout"`
    WriteTimeout       time.Duration `mapstructure:"write_timeout"`
    IdleTimeout        time.Duration `mapstructure:"idle_timeout"`
    ReadHeaderTimeout  time.Duration `mapstructure:"read_header_timeout"`
    MaxHeaderBytes     int           `mapstructure:"max_header_bytes"`
}

type PerformanceConfig struct {
    MaxProcs      int   `mapstructure:"max_procs"`
    GCPercent     int   `mapstructure:"gc_percent"`
    MemoryLimitMB int   `mapstructure:"memory_limit_mb"`
}
```

启动时应用：

```go
if cfg.Performance.MaxProcs > 0 {
    runtime.GOMAXPROCS(cfg.Performance.MaxProcs)
}
if cfg.Performance.GCPercent > 0 {
    debug.SetGCPercent(cfg.Performance.GCPercent)
}
if cfg.Performance.MemoryLimitMB > 0 {
    debug.SetMemoryLimit(int64(cfg.Performance.MemoryLimitMB) * 1024 * 1024)
}
```

## 附录 D：参考资料

- Go 官方 GC 指南：https://tip.golang.org/doc/gc-guide
- Go runtime pprof：https://pkg.go.dev/runtime/pprof
- GOMEMLIMIT 提案：https://go.dev/ref/spec#Go_1.19
- Gin 性能优化：https://gin-gonic.com/docs/faq/
- fasthttp 设计：https://github.com/valyala/fasthttp
- sonic 基准：https://github.com/bytedance/sonic#performance

---

## 总结：黄金三连口诀

> **分配少一点，超时短一点，堆小一点。**

1. **少分配**：sync.Pool、结构体替代 map、预分配 slice、sonic 替换 encoding/json
2. **短超时**：每个 ctx 都带 timeout，DB/Redis/HTTP 三个层次
3. **小堆稳**：GOMEMLIMIT 卡硬上限，GOGC 放宽到 200~400

做到这三条，Go 后端 P99 通常在 5 ms 以内；做不到，性能毛刺会反复出现，且难以定位根因（因为三者互相掩盖）。
