# 秒杀系统 - 技术点审查清单

> 逐条过，能讲清楚原理 + 手写核心代码 = ✅，讲不清楚或写不出来 = 复习重点。

---

## 一、架构设计

- [ ] 为什么秒杀的核心流程是「限流→Redis→Kafka→MySQL」而不是「直接写库」
- [ ] 每一层各自解决什么问题：限流(挡无效请求)、Redis(原子扣库存)、Kafka(削峰)、MySQL(持久化)
- [ ] 为什么库存预热到 Redis 而不是秒杀时查 MySQL
- [ ] 为什么扣减成功 ≠ 下单成功：Stock 和 Order 是两个独立操作
- [ ] 图片中「Redis 抗读、MQ 削峰写、DB 只做持久化」这句话，为什么要这样分层
- [ ] 流量漏斗：10000 QPS 请求 → 限流 → Redis → Kafka → MySQL，每层削减了多少

## 二、MySQL 设计

- [ ] 三张表：seckill_sessions / seckill_items / seckill_orders
- [ ] UNIQUE(user_id, item_id) 一人一单的数据库兜底
- [ ] DECIMAL(10,2) 为什么不用 FLOAT 存价格
- [ ] ENGINE=InnoDB + utf8mb4 的作用
- [ ] GORM AutoMigrate vs 手动 SQL，什么时候用什么
- [ ] OnConflict DoNothing：INSERT 重复时什么都不做，用于幂等

## 三、Redis 核心使用

- [ ] 库存预热：`SET seckill:stock:1 100`，为什么不设 TTL
- [ ] 已购集合：`SADD seckill:bought:1 {user_id}`，SCARD 对账用
- [ ] Redis 三种数据结构在这个系统里的用途：String(库存)、Set(已购)、Pub/Sub(未用但已知)
- [ ] 为什么库存预热不设 TTL：对账需要拿 Redis 剩余库存比对
- [ ] Redis 单线程模型：为什么 Lua 脚本天然原子，不需要加锁
- [ ] Redis 的持久化：RDB vs AOF，秒杀场景应该用哪个

## 四、Lua 脚本 — 最核心

- [ ] 手写 seckill.lua：sismember → get → decr → sadd 四步
- [ ] 为什么用 Lua 而不是「GET + 判断 + SET」三次 Redis 命令
- [ ] 三次命令之间的并发窗口：A 和 B 同时 GET 到 stock=1，两个都扣成 0 → 超卖
- [ ] Lua 脚本在 Redis 单线程中执行，天然隔离并发
- [ ] 三个返回值：1(成功) / 0(库存不足) / -1(已购买)
- [ ] KEYS[1] KEYS[2] ARGV[1] 参数传递方式
- [ ] EVAL vs EVALSHA：为什么生产环境用 EVALSHA 缓存 SHA
- [ ] SCRIPT LOAD 预加载 + EVALSHA 减少网络传输

## 五、一人一单

- [ ] Redis Set 记录已购买用户：`seckill:bought:{item_id}`
- [ ] Lua 脚本中 sismember 检查 → 已购买直接返回 -1
- [ ] MySQL UNIQUE(user_id, item_id) 兜底
- [ ] 为什么需要两层防护（Redis + MySQL）
- [ ] 如果 Redis Set 数据丢失怎么办？→ MySQL 唯一约束兜底

## 六、不超卖 — 并发安全

- [ ] 为什么直接「查库存→扣库存」会超卖（两次命令之间的竞态窗口）
- [ ] Lua 脚本如何解决：整个「查→扣→记」逻辑原子执行
- [ ] 并发测试：100 goroutine 抢 50 库存，成功数 = 50
- [ ] go test -race 检测数据竞态
- [ ] mock Redis 为什么需要 Mutex，真实 Redis 为什么不需要
- [ ] Kafka Consumer 写 MySQL 重复消息的幂等处理

## 七、Kafka 异步下单

- [ ] Producer 投递时机：Lua 返回 1(成功) 之后，不阻塞用户响应
- [ ] 消息体：`{"user_id":1,"item_id":1}`，JSON 序列化
- [ ] 分区 key = user_id：为什么同一用户进同一分区（顺序保证）
- [ ] 为什么用 Kafka 而不是同步写 MySQL：削峰 + 解耦
- [ ] Producer 配置：RequiredAcks / Retry.Max / Idempotent
- [ ] Consumer 手动提交 offset：写 MySQL 成功后才 commit，crash 可重试
- [ ] 重复消费幂等：MySQL UNIQUE 约束 + GORM OnConflict DoNothing
- [ ] Kafka vs RabbitMQ 选型：高吞吐 vs 复杂路由
- [ ] Kafka 为什么能百万 QPS：顺序写磁盘 + 零拷贝 + PageCache
- [ ] ISR（In-Sync Replicas）机制保证消息不丢

## 八、限流

- [ ] 固定窗口算法：计数器 + 窗口起始时间
- [ ] Key = IP + Path，每个用户独立计数
- [ ] 超限返回 429 Too Many Requests
- [ ] sync.Mutex 保护计数器并发安全
- [ ] 限流放在最外层：阻止无效请求穿透到 Redis
- [ ] 固定窗口的边界突刺问题：窗口切换瞬间可能双倍流量
- [ ] 限流 vs 降级 vs 熔断 三者的区别

## 九、核心接口

- [ ] POST /seckill/buy：接收 X-User-Id header + JSON body
- [ ] 为什么用 X-User-Id 模拟用户而不是 JWT（简化设计，聚焦秒杀逻辑）
- [ ] GET /seckill/result/:id：返回 bought + stock
- [ ] SIsMember 查询是否已购买
- [ ] Get 查询剩余库存

## 十、三层架构

- [ ] Handler → Service → Repository 各层职责
- [ ] Repository 接口定义：Session / Item / Order / RedisClient / RedisCmdClient
- [ ] Service 依赖 Repository 接口，不依赖具体实现
- [ ] 为什么 RedisClient 和 RedisCmdClient 分开定义
- [ ] go:embed 嵌入 Lua 脚本到 Go 二进制

## 十一、测试体系

- [ ] Mock Redis：实现 RedisCmdClient 接口，内存 map 模拟
- [ ] Mock Kafka Producer/Consumer：内存队列模拟
- [ ] 并发测试：WaitGroup + atomic 计数 + mutex 保护 mock
- [ ] 成功路径测试：库存充足 → 扣减成功 → 返回 1
- [ ] 失败路径测试：库存为 0 → soldout / 已购买 → already_bought
- [ ] 边界测试：price > origin_price 拒绝 / 空标题拒绝 / end < start 拒绝
- [ ] 并发安全测试：100 goroutine 抢 50 库存，成功数 = 50
- [ ] E2E 测试脚本：预热 → 抢购 → 重复 → 查询 全链路

## 十二、Docker 工程化

- [ ] Docker Compose 四容器编排：app + MySQL + Redis + Kafka
- [ ] 自定义网络 + DNS 服务发现（`mysql` / `redis` / `kafka` 作为 hostname）
- [ ] depends_on + healthcheck：等 MySQL/Redis 就绪后才启动 app
- [ ] Kafka KRaft 模式：去掉 ZooKeeper 依赖（Kafka 3.3+）
- [ ] 多阶段构建：编译阶段(golang:1.25) → 运行阶段(alpine)
- [ ] CGO_ENABLED=0 + GOOS=linux 交叉编译
- [ ] vendor 离线构建：Docker 内无需下载 Go 依赖
- [ ] Go 版本一致性：go.mod 和 Docker 镜像版本必须匹配

## 十三、压测与对账

- [ ] wrk 压测：-t12 -c400 -d30s
- [ ] 压测前预热库存
- [ ] 压测后对账：Redis 库存 + MySQL 订单数 = 初始库存
- [ ] 验证不超卖：实际售出 ≤ 设定库存
- [ ] 验证一人一单：Redis Set 大小 = MySQL 订单数
- [ ] P99 延迟目标的含义

## 十四、Go 语言技巧

- [ ] interface 定义契约：Repository 接口 + mock 实现
- [ ] `//go:embed seckill.lua` 嵌入文件到二进制
- [ ] context.Context 传递请求上下文
- [ ] sync.WaitGroup + sync.Mutex + sync/atomic 并发控制
- [ ] `:=` vs `=` 变量遮盖（在秒杀中踩过 router.go 的坑）
- [ ] GORM tag：OnConflict DoNothing / UNIQUE 约束
- [ ] Gin ShouldBindJSON 参数校验
- [ ] c.GetHeader / c.Param / c.Query 获取请求参数

## 十五、面试场景模拟

- [ ] "秒杀系统的核心难点是什么，你怎么解决的"
- [ ] "如果不用 Lua 脚本，还有什么办法保证不超卖"
- [ ] "Redis 库存扣完了但用户没收到成功通知怎么办"
- [ ] "Kafka 消息丢了怎么办，怎么保证消息可靠性"
- [ ] "为什么用固定窗口限流，不用滑动窗口或令牌桶"
- [ ] "如果秒杀流量再大 10 倍，当前架构哪里会先撑不住"
- [ ] "你都做了哪些测试，怎么证明系统是正确的"
- [ ] "为什么用 MySQL 而不是 PostgreSQL"
- [ ] "一人一单是怎么保证的，如果 Redis 宕机了怎么办"
- [ ] "你的 QPS 瓶颈在哪里，怎么优化"

## 十六、和博客系统的能力对照

| 能力 | 博客 | 秒杀 | 新增掌握 |
|---|---|---|---|
| 数据库 | PostgreSQL | **MySQL 8.0** | 两种 DSN + 两种 SQL 方言 |
| 缓存 | 仅限流 | **Lua 原子库存** | Redis 进阶用法 |
| 消息队列 | 无 | **Kafka 削峰** | Producer/Consumer/Offset |
| 并发 | 无 | **Goroutine 竞态** | Mutex/WaitGroup/Race |
| 性能 | 无要求 | **QPS 10000+** | wrk 压测 + 对账 |
| 部署 | 3 容器 | **4 容器** | Kafka KRaft 模式 |

---

## 复习建议

1. **第一优先级**：第四章 Lua 脚本 — 手写 + 解释每一行
2. **第二优先级**：第七章 Kafka — 完整 Producer → Consumer 链路
3. **第三优先级**：第十三章压测 — 跑一次 wrk + 对账，记录 QPS 数据
4. **面试前**：对着第十五节的 10 个问题，用手机录音自问自答
