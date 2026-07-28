# API 网关 - 技术点审查清单

> 逐条过，能讲清楚原理 + 手写核心代码 = ✅，讲不清楚或写不出来 = 复习重点。

---

## 一、架构设计

- [ ] 网关和业务服务的本质区别：网关是基础设施，不处理业务逻辑
- [ ] 网关的职责：路由转发、认证鉴权、限流、熔断、日志
- [ ] 为什么要把认证/限流放到网关而不是每个服务自己做
- [ ] 网关是七层(HTTP)代理，和四层(TCP)负载均衡的区别
- [ ] 请求在网关的完整路径：限流 → 认证 → 路由匹配 → 代理转发 → 上游
- [ ] 插件链的执行顺序：Priority 高的先执行
- [ ] 为什么插件链用洋葱模型（后进先出反向构建）

## 二、反向代理

- [ ] `httputil.ReverseProxy` 的核心原理：修改 Request 的 Host 和 Path
- [ ] `NewSingleHostReverseProxy(target *url.URL)` 的用法
- [ ] `proxy.ServeHTTP(w, r)` —— 这行代码做了什么
- [ ] ErrorHandler：上游不可达时返回 502 Bad Gateway
- [ ] 为什么用 `r.URL.Path = remainder` 而不是修改原始请求
- [ ] 为什么对上游请求要设置 `r.Host = target.Host`
- [ ] `httptest.NewServer` 模拟上游：网关测试的核心技巧
- [ ] 反向代理 vs 正向代理：方向区别

## 三、路由表

- [ ] 最长前缀匹配：`/blog/admin` 优先于 `/blog`
- [ ] `strings.HasPrefix` + `strings.TrimPrefix` 组合用法
- [ ] 前缀剥离：`/blog/api/v1/ping` → 上游收到 `/api/v1/ping`
- [ ] 为什么不剥离前缀：上游服务按完整路径注册路由
- [ ] 路由表的线程安全：如果有热更新，需要 `sync.RWMutex`
- [ ] 配置文件中路由的定义格式：path / upstream / keep_prefix

## 四、JWT 认证

- [ ] 网关层 JWT 认证：统一入口，后端服务不再各自认证
- [ ] 解析 JWT → 提取 user_id → 注入 `X-User-Id` Header 传给上游
- [ ] 为什么用 Header 注入而不是 URL 参数
- [ ] 白名单路径免认证：`/health`、`/admin`
- [ ] 认证失败的三种情况：无 token / 格式错 / 过期或无效
- [ ] Bearer 前缀的含义
- [ ] `strings.SplitN(header, " ", 2)` 正确解析 Authorization

## 五、限流算法 — 面试必考

### 固定窗口

- [ ] 原理：每个时间窗口维护一个计数器
- [ ] 窗口过期判断：`now.Sub(entry.window) > window`
- [ ] 窗口边界的突刺问题：窗口切换瞬间可能双倍流量
- [ ] 实现：map[string]*entry + 锁保护

### 滑动窗口

- [ ] 原理：记录每次请求的时间戳，统计最近 N 秒内的请求数
- [ ] 旧请求滑出：`times[i].After(time.Now().Add(-window))`
- [ ] 为什么比固定窗口精确：没有窗口边界突刺
- [ ] 空间开销：每个 key 维护一个时间戳数组

### 令牌桶

- [ ] 原理：以固定速率向桶中放令牌，请求消费令牌
- [ ] 令牌补充公式：`tokens += elapsed * rate`
- [ ] 为什么允许突发：桶中可以积攒令牌
- [ ] capacity 参数的作用：最大积攒令牌数

### 漏桶

- [ ] 原理：请求先入队，以固定速率出队处理
- [ ] 为什么能流量整形：出队速率恒定
- [ ] water 变量的含义：当前积压的请求量
- [ ] 溢出处理：积压超过 capacity → 拒绝

### 四种算法对比

| 算法 | 平滑度 | 允许突发 | 实现复杂度 | 适用场景 |
|---|---|---|---|---|
| 固定窗口 | 差 | 短时 | 简单 | 简单限流 |
| 滑动窗口 | 好 | 否 | 中等 | 精确限流 |
| 令牌桶 | 中 | 是 | 中等 | 允许弹性 |
| 漏桶 | 好 | 否 | 简单 | 流量整形 |

## 六、熔断器

- [ ] 三种状态：Closed(正常) → Open(熔断) → HalfOpen(试探)
- [ ] Closed → Open：连续 N 次错误
- [ ] Open → HalfOpen：冷却时间到
- [ ] HalfOpen → Closed：试探请求成功
- [ ] HalfOpen → Open：试探请求失败
- [ ] `sync.Mutex` 保护状态切换的并发安全
- [ ] 为什么熔断器要拒绝所有请求而不是排队
- [ ] 熔断、降级、限流三者的区别
- [ ] 面试话术：解释你实现的熔断器状态机全路径

## 七、动态配置热更新

- [ ] `fsnotify` 监听文件变化
- [ ] `NewWatcher()` → `Add(path)` → `select events`
- [ ] 热更新回调：重新 Load 配置 → 重建路由表
- [ ] `sync.RWMutex` 保护配置读写
- [ ] 为什么不用定时轮询：fsnotify 是事件驱动，更实时
- [ ] 热更新的原子性：先解析新配置成功，再原子替换

## 八、插件架构

- [ ] Plugin 接口三要素：Name / Priority / Handle
- [ ] Handle 的签名：`func(http.Handler) http.Handler` — 标准 HTTP 中间件
- [ ] Registry.Register：注册插件
- [ ] Registry.GetChain：按 Priority 降序排列
- [ ] BuildChain：从后往前构建中间件链
- [ ] 为什么从后往前构建：最内层是最终 handler，往外逐层包装
- [ ] Priority 高的先执行（放在最外层）
- [ ] 插件中断链：不调用 next.ServeHTTP 即可

**BuildChain 的构建过程：**

```go
// plugins sorted: [Auth:100, Rate:200, Log:50]
// final: proxyHandler
// step 1: Log(proxyHandler)
// step 2: Rate(Log(proxyHandler))
// step 3: Auth(Rate(Log(proxyHandler)))
// 执行顺序: Auth → Rate → Log → proxyHandler
```

## 九、Go 语言要点

- [ ] `net/http/httputil.ReverseProxy` 的 API
- [ ] `httptest.NewServer` 模拟 HTTP 服务
- [ ] `http.HandlerFunc` 类型转换
- [ ] `strings.HasPrefix` / `strings.TrimPrefix` / `strings.SplitN`
- [ ] `sort.Slice` 自定义排序
- [ ] `sync.RWMutex` 读写锁
- [ ] `copy(dst, src)` 浅拷贝 slice
- [ ] `yaml.Unmarshal` 解析 yaml 配置
- [ ] `fsnotify.NewWatcher` 文件监听
- [ ] `flag.String` 命令行参数
- [ ] `r.Header.Set("X-User-Id", ...)` 注入 Header

## 十、测试策略

- [ ] 路由表测试：纯单元，零依赖
- [ ] 限流算法测试：纯单元，每个算法 2-4 条独立测试
- [ ] 熔断器测试：纯单元，所有状态转换路径
- [ ] 插件测试：纯单元 + httptest，验证链式执行和中断
- [ ] 代理测试：httptest 模拟上游，验证转发和错误处理
- [ ] E2E 测试：Docker 环境，多上游全链路

## 十一、与前三个项目的区别

| | 博客/秒杀/IM | API 网关 |
|---|---|---|
| 项目类型 | 业务服务 | **通用基础设施** |
| 核心库 | gin/gorm | **httputil.ReverseProxy** |
| 新增知识点 | — | **4种限流 + 熔断器 + 插件化** |
| 测试模式 | 业务逻辑 | **httptest 模拟上游** |

## 十二、面试模拟

- [ ] "你实现的网关和 Nginx/Kong 有什么区别？"
- [ ] "四种限流算法分别适用于什么场景？你生产推荐哪个？"
- [ ] "熔断器的三种状态是怎么切换的？"
- [ ] "怎么实现路由热更新而不丢请求？"
- [ ] "网关怎么保证高可用？单点故障怎么办？"
- [ ] "JWT 认证放在网关层有什么好处和风险？"
- [ ] "你的插件架构和 Gin 中间件有什么异同？"
- [ ] "如果上游服务响应很慢，网关怎么处理？"

---

## 复习建议

1. **第一优先级**：第五章 4种限流算法 — 手写每个算法的 Allow 函数
2. **第二优先级**：第六章熔断器 — 手写状态机全路径转换
3. **第三优先级**：第八章插件 BuildChain — 理解从后往前构建
4. **面试前**：对着第十二节 8 个问题自问自答
