# 实时 IM 系统

**技术栈：** Go + WebSocket + Redis Pub/Sub + Kafka + MySQL + 一致性哈希 + Docker

**项目描述：** 多节点分布式即时通讯系统。支持水平扩展，跨节点消息实时路由，离线消息异步投递，7+ 容器 Docker Compose 编排。

**核心亮点：**
- **多节点 WebSocket 架构**：1 个 Gateway（认证 + 负载分配） + 3 个 IM Node（长连接管理），节点之间通过 Redis Pub/Sub 路由消息
- **一致性哈希**：CRC32 + 150 个虚拟节点，用户 → 节点映射，同一用户始终连接同一节点，避免跨节点状态同步
- **消息路由三级策略**：在线且同节点 → 直推；在线跨节点 → Redis Pub/Sub 转发；离线 → Kafka 持久化 + 消费者异步投递
- **连接管理**：Read Pump / Write Pump 分离，30s Ping / 90s 超时断连，max 10000 连接限制
- **并发安全**：`sync.RWMutex` 保护连接表，channel 解耦读写协程

**工程实践：** 7 容器 Docker Compose（Gateway × 1 + IM Node × 3 + Redis + MySQL + Kafka）| 真实 WebSocket 握手测试

**测试：** 27 条测试（WebSocket 16 + 一致性哈希 4 + JWT 5 + Kafka 2）
