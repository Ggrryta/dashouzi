# API 网关

**技术栈：** Go + `httputil.ReverseProxy` + Gin + JWT + Docker

**项目描述：** 自研 API 网关，统一微服务入口。实现路由转发、四种限流算法、熔断器、JWT 认证、插件链等基础设施能力。非调库项目，全部核心逻辑手写。

**核心亮点：**
- **四种限流算法手写**：固定窗口、滑动窗口、令牌桶、漏桶，理解「为什么限流」到「每种算法优缺点」
- **熔断器状态机**：Closed → Open → HalfOpen，连续错误数阈值触发。Open 状态下直接拒绝，HalfOpen 放少量探测请求
- **插件洋葱模型**：Plugin 接口 + BuildChain 构建调用链。Auth → RateLimit → Route → Proxy 像洋葱层层包裹
- **`httputil.ReverseProxy` 零拷贝转发**：利用 Go 标准库，修改 Director 函数实现路径改写、Header 透传、上游不可达返回 502
- **热更新路由表**：Viper + fsnotify 监听配置文件变化，不重启网关即可新增路由规则

**工程实践：** httptest.Server 模拟上游测试 | 统一错误码 | 结构化日志

**测试：** 32 条测试（breaker 4 + limiter 15 + plugin 3 + proxy 5 + router 5）
