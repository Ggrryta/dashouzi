# IM 聊天系统 - 技术点审查清单

> 逐条过，能讲清楚原理 + 手写核心代码 = ✅，讲不清楚或写不出来 = 复习重点。

---

## 一、架构设计

- [ ] 为什么 IM 要两个独立进程（Gateway + IM Node）而不是一个
- [ ] Gateway 的职责：认证（JWT）+ 节点分配（一致性哈希）
- [ ] IM Node 的职责：WebSocket 连接管理 + 消息路由
- [ ] 为什么三个 IM Node 是同一套代码部署三次，不需要写三份
- [ ] HTTP 短连接 vs WebSocket 长连接：各自的适用场景
- [ ] 服务端主动推送 vs 客户端轮询：为什么 WebSocket 是最优解
- [ ] 七容器分别是什么，各自的作用

## 二、WebSocket 协议

- [ ] HTTP Upgrade 握手：101 Switching Protocols
- [ ] 客户端如何发起 WebSocket 连接：`ws://host:port/path?token=xxx`
- [ ] gorilla/websocket 的 CheckOrigin 为什么必须处理
- [ ] `conn.ReadMessage()` vs `conn.WriteMessage()` 的阻塞特性
- [ ] Ping/Pong 帧：协议级心跳 vs 应用层心跳
- [ ] gorilla 自动回复 Pong：`SetPongHandler`
- [ ] `SetReadDeadline` / `SetWriteDeadline` 的作用
- [ ] WebSocket 关闭握手：CloseMessage 的正确发送方式

## 三、连接管理（ConnManager）

- [ ] 为什么要读写 goroutine 分离：ReadPump + WritePump
- [ ] WritePump 从 Send channel 取消息写入 WebSocket
- [ ] ReadPump 从 WebSocket 读消息交给 Hub 路由
- [ ] Send channel 的作用：避免多个 goroutine 并发写 WebSocket
- [ ] Send channel 满了怎么办：`select default` 跳过不阻塞
- [ ] 同用户重复登录：关闭旧连接的 Send channel → 旧 WritePump 退出
- [ ] `sync.RWMutex` 为什么用读写锁而不是普通 Mutex（读多写少）
- [ ] ConnManager.Add / Get / Remove 的并发安全
- [ ] 为什么 100 goroutine 并发 Add/Remove/Get 不能 panic

## 四、心跳与在线状态

- [ ] 30s Ping / 90s 无 Pong 断开：参数选择依据
- [ ] `time.NewTicker` 心跳定时器
- [ ] `SetPongHandler` 重置 ReadDeadline
- [ ] 用户断开后：ConnManager.Remove + close(Send) + conn.Close()
- [ ] 为什么要在 Redis 存在线状态而不是内存
- [ ] 多节点之间怎么知道用户在哪个节点：查 Redis `im:online`
- [ ] 离线检测：定时扫描心跳时间 → 超时则 HDel

## 五、消息协议

- [ ] Message JSON 结构：type / from / to / content / msg_id
- [ ] type 字段区分消息类型：msg / ack / ping
- [ ] msg_id 为什么用 UUID：全局唯一 + ACK 回执 + 离线去重
- [ ] `json.Unmarshal` 非法 JSON 的处理
- [ ] 消息如何从客户端 → Hub → 接收方：完整链路

## 六、Hub 消息路由

- [ ] Hub.HandleMessage 先判断 type：ack → handleACK / 其他 → handleChat
- [ ] handleChat 三步：① 存 DB → ② 本地推送 → ③ 跨节点 Pub/Sub
- [ ] 为什么要先存 DB 再推送：保证消息不丢
- [ ] 存 DB 失败 → 不推送（原子性）
- [ ] 接收方在本节点 → 直接送到 Send channel
- [ ] 接收方不在本节点 → 查 Redis online → Pub/Sub 发布到目标节点
- [ ] 接收方不在线 → 消息保持 pending（等 Day 5 离线推送）
- [ ] Hub 为什么依赖接口 MessageRepo 而不是具体实现

## 七、ACK 回执机制

- [ ] ACK 消息格式：`{"type":"ack","msg_id":"uuid"}`
- [ ] 收到 ACK → `UpdateStatus(msgID, "delivered")`
- [ ] 为什么通知发送方而不是接收方
- [ ] 发送方怎么知道消息已送达：收到 ACK 回执
- [ ] 发送方离线怎么办：ACK 存 DB，上线后查询
- [ ] ACK 丢失怎么办：DB 中 status 保持 pending，对账兜底

## 八、一致性哈希

- [ ] 为什么不用 `hash(userID) % N`：节点增减导致大量用户重新分配
- [ ] 哈希环原理：节点映射到环上多个虚拟节点
- [ ] Add：每个物理节点映射 150 个虚拟节点到环上
- [ ] Get：计算 key 的 hash → 顺时针找到最近的虚拟节点 → 返回物理节点
- [ ] Remove：从环上删除节点的所有虚拟节点
- [ ] 为什么 150 个虚拟节点：分布均匀性的经验值
- [ ] 删除节点后迁移率 < 70%（普通 hash 100%）
- [ ] 如何验证分布均匀性：10000 个 key 统计各节点分配比例
- [ ] `sort.Search` 二分查找的使用

## 九、Redis Pub/Sub 跨节点路由

- [ ] 为什么选 Pub/Sub 而不是 Kafka：实时的，不需要持久化
- [ ] 每个 IM Node 订阅自己的频道：`im:node:{node_id}`
- [ ] Publish：`redis.Publish("im:node:node-2", msg)`
- [ ] Subscribe：`redis.Subscribe("im:node:node-1")` → goroutine 阻塞读
- [ ] 为什么 Pub/Sub 消息不持久化：连接断了消息就丢了
- [ ] 节点注册：`HSet("im:nodes", nodeID, addr)`
- [ ] 在线状态：`HSet("im:online", userID, nodeID)`
- [ ] 离线状态：`HDel("im:online", userID)`

## 十、Kafka 离线消息

- [ ] 离线消息 topic 命名：`im:offline:user_{userID}`
- [ ] 接收方不在线 → 投递 Kafka
- [ ] 用户上线 → Consumer 拉取该用户的离线 topic → 逐条推送
- [ ] 推送后 commit offset → 消息不丢
- [ ] 为什么不直接在 Redis Pub/Sub 做离线：Pub/Sub 不持久化
- [ ] 为什么离线消息用 Kafka 而不是 MySQL 轮询：实时性 + 解耦
- [ ] 一条消息可能被重复消费吗？怎么处理（msg_id 去重）

## 十一、消息可靠性

- [ ] 消息从发出到对方收到的完整路径
- [ ] 每一步可能丢消息的点：
  - ① WebSocket 连接断开
  - ② DB 写入失败
  - ③ Redis Pub/Sub 消息丢失（节点宕机）
  - ④ Kafka Consumer 崩溃
- [ ] 每一步的对策：
  - ① 心跳检测 + 断线重连
  - ② 先存 DB 再推送
  - ③ Pub/Sub 丢了不影响（消息已存 DB，对账恢复）
  - ④ Consumer 写成功后才 commit offset

## 十二、测试策略（最小 Mock 方案）

- [ ] 纯逻辑零依赖测试：JWT、一致性哈希、消息协议、ConnManager
- [ ] 仅用真实 goroutine + channel 测试：Send channel 不阻塞
- [ ] Hub 测试：mock MessageRepo（内存实现），真 ConnManager + goroutine
- [ ] 离线测试：mock Kafka Producer/Consumer（内存队列）
- [ ] 为什么允许 mock Kafka：异步离线非实时路径，接口验证即可
- [ ] 集成测试走 Docker：Redis Pub/Sub + Kafka + MySQL + WebSocket
- [ ] 并发安全测试：100 goroutine ConnManager + mutex 保护 mock Redis

## 十三、Go 语言要点

- [ ] `interface{}` 定义契约：MessageRepo / Producer / Consumer
- [ ] goroutine + channel 并发模型：读写分离
- [ ] `sync.RWMutex` vs `sync.Mutex`：读多写少的优化
- [ ] `select` 多路复用：Send channel + ticker 心跳
- [ ] `close(channel)` 通知 goroutine 退出
- [ ] `context.Context` 传递请求上下文
- [ ] `flag.Parse` + `flag.String` 命令行参数
- [ ] `json.Unmarshal` 错误处理
- [ ] `//go:embed` 嵌入文件到二进制（seckill 用过）
- [ ] `sort.Search` 二分查找
- [ ] `hash/crc32` 哈希算法

## 十四、WebSocket 特定问题

- [ ] 并发写 WebSocket 的后果：panic
- [ ] Send channel 如何解决并发写问题
- [ ] 连接关闭时 Send channel 被 close → WritePump 收到 zero value → 发送 CloseMessage
- [ ] goroutine 泄漏：ReadPump 不退出导致 goroutine 堆积
- [ ] 优雅关闭：`defer mgr.Remove` + `defer conn.Close` + `defer close(Send)`

## 十五、Docker 工程化

- [ ] 同一个 Dockerfile 构建两个不同二进制
- [ ] docker-compose 中用 `entrypoint` + `command` 区分 gateway 和 node
- [ ] 环境变量覆盖配置：NODE_ID / SERVER_PORT
- [ ] 同一套代码 + 不同端口 + 不同环境变量 = 3 个独立节点
- [ ] depends_on + healthcheck：等 MySQL/Redis ready
- [ ] Docker Desktop DNS 波动问题及绕过方案（extra_hosts / links）

## 十六、面试场景模拟

- [ ] "你做的 IM 系统架构是什么样的，几层？"
- [ ] "用户 A 发消息给用户 B，完整路径是什么？"
- [ ] "A 和 B 不在同一个节点上，消息怎么送达？"
- [ ] "一致性哈希和普通哈希有什么区别？为什么选一致性哈希？"
- [ ] "Redis Pub/Sub 能保证消息不丢吗？不能怎么办？"
- [ ] "WebSocket 连接断了，消息会丢吗？"
- [ ] "怎么保证消息有序？"
- [ ] "一个节点挂了，影响范围有多大？用户会断连吗？"
- [ ] "如果扩展到 100 个节点，当前架构有什么瓶颈？"
- [ ] "你的消息可靠性是怎么保证的？"

## 十七、与博客、秒杀的能力对照

| 能力 | 博客 | 秒杀 | IM |
|---|---|---|---|
| 协议 | HTTP | HTTP | HTTP + **WebSocket** |
| 节点数 | 1 | 1 | **1 Gateway + 3 Node** |
| 连接模型 | 短连接 | 短连接 | **长连接（状态绑定）** |
| Redis 用法 | 限流 | Lua 原子库存 | **Pub/Sub + Hash 在线状态** |
| Kafka 用法 | — | 异步下单 | **离线消息投递** |
| 新算法 | — | — | **一致性哈希** |
| 并发模型 | — | goroutine 竞态 | **读写 goroutine 分离** |

---

## 复习建议

1. **第一优先级**：第六章 Hub 路由——画出消息从 A 到 B 的完整路径
2. **第二优先级**：第八章一致性哈希——手写 Add/Get/Remove
3. **第三优先级**：第三章连接管理——手写 ReadPump + WritePump
4. **面试前**：对着第十六节的 10 个问题，画架构图自问自答
