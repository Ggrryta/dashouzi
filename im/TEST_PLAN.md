# IM 聊天系统 - 5 天测试计划（最小 Mock 版）

> 原则：**能不 mock 就不 mock。** 核心组件（Redis Pub/Sub、Kafka、MySQL、WebSocket）全部走真实 Docker。
> 只有两类场景允许 Fake：纯算法（一致性哈希）和纯数据结构（ConnManager 用真实 goroutine+channel）。

---

## Mock/Fake/真实 分界

```
                    ┌─── 真实 Docker 集成测试 ───┐
                    │  Redis Pub/Sub 跨节点       │
                    │  Kafka 离线消息投递         │
                    │  MySQL 消息持久化           │
                    │  WebSocket 真实连接         │
                    └─────────────────────────────┘

                    ┌─── Fake（内存等价实现）─────┐
                    │  ConnManager：真实 channel  │
                    │  Hub 路由：真实 goroutine   │
                    └─────────────────────────────┘

                    ┌─── 纯逻辑（零依赖）─────────┐
                    │  一致性哈希                 │
                    │  消息协议 JSON 序列化       │
                    └─────────────────────────────┘
```

**红线：Redis、Kafka、MySQL、WebSocket 这四个绝不 mock。**

---

## Day 1 测试计划：骨架 + WebSocket 握手

**对应开发：** Day 1

| # | 测试内容 | 层级 | 环境 |
|---|---|---|---|
| T1.1 | JWT 签发 + 正确解析 | 单元 Small | 零依赖 |
| T1.2 | JWT 过期 / 篡改 / 错误密钥 | 单元 Small | 零依赖 |
| T1.3 | Gateway 注册 + 登录返回 token + 节点地址 | 集成 Medium | **真 Gateway + 真 MySQL** |
| T1.4 | WebSocket 升级成功（带有效 token） | 集成 Medium | **真 IM Node** |
| T1.5 | 无效 token → WS 连接被拒绝 | 集成 Medium | **真 IM Node** |
| T1.6 | 无 token → WS 连接被拒绝 | 集成 Medium | **真 IM Node** |
| T1.7 | Ping → 自动回复 Pong（协议级心跳） | 集成 Medium | **真 WebSocket 连接** |
| T1.8 | Docker 七容器 healthcheck 全绿 | E2E Large | Docker |

### Day 1 核心用例

```go
func TestJWT_GenerateAndParse(t *testing.T) {
    j := jwt.New("secret", 24)
    token, _ := j.GenerateToken(1, "alice")
    claims, err := j.ParseToken(token)
    // 零依赖，纯逻辑
    assert.NoError(t, err)
    assert.Equal(t, uint(1), claims.UserID)
}

// 集成测试：运行在 Docker 内
func TestWebSocket_UpgradeAndPingPong(t *testing.T) {
    // 连接真实 IM Node
    conn, _, err := websocket.DefaultDialer.Dial(
        "ws://im-node-1:8081/ws?token="+validToken, nil)
    require.NoError(t, err)
    defer conn.Close()

    // 写 Ping → 读 Pong
    conn.WriteMessage(websocket.PingMessage, nil)
    _, msg, _ := conn.ReadMessage()
    // gorilla 自动回复 Pong，协议层已验证
    assert.NotNil(t, msg)
}
```

### Day 1 验收

```bash
# 单元测试（零依赖，秒级）
go test ./pkg/jwt/ -v -count=1

# 集成测试（Docker 内）
docker exec im_node_1 go test -tags=integration ./internal/ws/ -run "TestWebSocket"

# E2E
docker compose ps  # 7 个服务全 healthy
```

---

## Day 2 测试计划：连接管理 + 在线状态

**对应开发：** Day 2

| # | 测试内容 | 层级 | 环境 |
|---|---|---|---|
| T2.1 | ConnManager Add / Get / Remove | 单元 Small | 真实 goroutine + channel |
| T2.2 | ConnManager Get 不存在 → nil | 单元 Small | 同上 |
| T2.3 | 同用户重复登录 → 旧连接被踢（Send channel 关闭） | 单元 Small | 同上 |
| T2.4 | Send channel 满 → 跳过不阻塞 | 单元 Small | 同上 |
| T2.5 | 100 goroutine 并发 Add/Remove/Get → 不 panic | 单元 Small | 同上 |
| T2.6 | 用户登录 → Redis HSet `im:online` | 集成 Medium | **真 IM Node + 真 Redis** |
| T2.7 | 用户断开 → Redis HDel `im:online` | 集成 Medium | **真 Redis** |
| T2.8 | 心跳 30s → 在线状态保持 | 集成 Medium | **真 WS 连接 + 真 Redis** |
| T2.9 | 90s 无心跳 → 自动清理（连验证） | 集成 Medium | **真 Redis** |

### Day 2 核心用例

```go
// 单元测试：零依赖
func TestConnManager_Concurrent(t *testing.T) {
    mgr := ws.NewConnManager()
    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(uid int) {
            defer wg.Done()
            mgr.Add(&ws.Client{UserID: uint64(uid), Send: make(chan []byte, 10)})
            mgr.Get(uint64(uid))
            mgr.Remove(uint64(uid))
        }(i)
    }
    wg.Wait()
}

// 集成测试：真 Redis
func TestOnlineStatus_RedisAfterConnect(t *testing.T) {
    rdb := redis.NewClient(&redis.Options{Addr: "redis:6379"})

    // 用户连接 IM Node
    conn := connectWS(t, "user-1")
    defer conn.Close()

    // Redis 中应存在在线记录
    val, err := rdb.HGet(ctx, "im:online", "user-1").Result()
    assert.NoError(t, err)
    assert.Equal(t, "node-1", val)

    // 断开后应清除
    conn.Close()
    time.Sleep(2 * time.Second)
    exists, _ := rdb.HExists(ctx, "im:online", "user-1").Result()
    assert.False(t, exists)
}
```

### Day 2 验收

```bash
go test ./internal/ws/ -v -run "TestConnManager"    # 纯逻辑
go test -tags=integration ./internal/ws/ -run "TestOnline"  # 真 Redis
```

---

## Day 3 测试计划：单节点消息 + ACK + 持久化

**对应开发：** Day 3

| # | 测试内容 | 层级 | 环境 |
|---|---|---|---|
| T3.1 | 消息协议 JSON 序列化/反序列化 | 单元 Small | 零依赖 |
| T3.2 | 消息写 MySQL → status=pending | 集成 Medium | **真 MySQL** |
| T3.3 | A 发消息 → B 在同一节点 → B 通过 WS 收到 | 集成 Medium | **真 IM Node + MySQL + Redis** |
| T3.4 | B 收到消息 → 自动回 ACK | 集成 Medium | **真 WS** |
| T3.5 | A 收到 ACK → MySQL 中 status=delivered | 集成 Medium | **真 MySQL** |
| T3.6 | 接收方不在线 → 消息存 MySQL pending | 集成 Medium | **真 MySQL** |
| T3.7 | 消息存储失败 → 不推送（原子性） | 集成 Medium | MySQL 故意断开测试 |
| T3.8 | 消息去重：同一 msg_id 重复发送 → 幂等 | 集成 Medium | **真 MySQL UNIQUE** |
| T3.9 | 同节点 E2E：两个真 WS 客户端互发消息 | E2E Large | **7 容器全链路** |

### Day 3 核心用例

```go
// 集成测试：真 WS + 真 MySQL
func TestMessage_SameNode_DeliveryAndAck(t *testing.T) {
    // A 连接 Node1
    connA := connectWS(t, "user-a")
    defer connA.Close()

    // B 也连 Node1
    connB := connectWS(t, "user-b")
    defer connB.Close()

    // A 发送消息给 B
    msg := `{"type":"msg","to":"user-b","content":"hello","msg_id":"uuid-1"}`
    connA.WriteMessage(websocket.TextMessage, []byte(msg))

    // B 应收到
    _, data, _ := connB.ReadMessage()
    var received map[string]interface{}
    json.Unmarshal(data, &received)
    assert.Equal(t, "hello", received["content"])

    // MySQL 中 status 应变为 delivered（ACK 后）
    time.Sleep(500 * time.Millisecond)
    var status string
    db.QueryRow("SELECT status FROM im_messages WHERE msg_id='uuid-1'").Scan(&status)
    assert.Equal(t, "delivered", status)
}
```

### Day 3 验收

```bash
go test -tags=integration ./internal/ws/ -run "TestMessage_SameNode" -v
# PASS: A 发 → B 收 → ACK → MySQL delivered
```

---

## Day 4 测试计划：跨节点 + Redis Pub/Sub + 一致性哈希

**对应开发：** Day 4

| # | 测试内容 | 层级 | 环境 |
|---|---|---|---|
| T4.1 | 一致性哈希：同用户始终同节点 | 单元 Small | 零依赖 |
| T4.2 | 一致性哈希：10000 用户均匀分布 | 单元 Small | 零依赖 |
| T4.3 | 一致性哈希：删除节点迁移率 < 70% | 单元 Small | 零依赖 |
| T4.4 | 节点注册：IM Node 启动 → Redis HSet | 集成 Medium | **真 Redis** |
| T4.5 | 网关根据一致性哈希分配用户到节点 | 集成 Medium | **真 Gateway + 真 Redis** |
| T4.6 | A 在 Node1 → 发消息给 B（在 Node2）→ **通过真实 Redis Pub/Sub 送达** | 集成 Medium | **真 Redis Pub/Sub** |
| T4.7 | B 回 ACK → 通过 Pub/Sub 回传给 Node1 | 集成 Medium | **真 Redis Pub/Sub** |
| T4.8 | Node2 宕机 → 一致性哈希漂移 → B 重连到 Node3 | 集成 Medium | **Docker stop** |
| T4.9 | 跨节点 + 异步 E2E | E2E Large | 7 容器 |

### Day 4 核心用例

```go
// 集成测试：真 Redis Pub/Sub
func TestPubSub_CrossNode_MessageDelivery(t *testing.T) {
    rdb := redis.NewClient(&redis.Options{Addr: "redis:6379"})

    // 模拟 Node2 订阅
    pubsub := rdb.Subscribe(ctx, "im:node:node-2")
    ch := pubsub.Channel()

    // Node1 发布消息到 Node2
    msg := `{"type":"msg","to":"user-b","content":"hello","msg_id":"uuid-2"}`
    rdb.Publish(ctx, "im:node:node-2", msg)

    // Node2 应在 1 秒内收到
    select {
    case received := <-ch:
        assert.Contains(t, received.Payload, "uuid-2")
    case <-time.After(time.Second):
        t.Fatal("pubsub message not received")
    }
}

// 集成测试：两个真实 IM Node + Redis Pub/Sub
func TestMessage_CrossNode(t *testing.T) {
    // A 通过 Gateway 分配到 Node1，连 ws://node-1:8081
    // B 通过 Gateway 分配到 Node2，连 ws://node-2:8082

    connA := connectToNode("node-1:8081", "user-a")
    connB := connectToNode("node-2:8082", "user-b")

    // A 发消息给 B
    connA.WriteMessage(websocket.TextMessage, []byte(msgToB))

    // B 通过 Redis Pub/Sub 跨节点收到
    _, data, _ := connB.ReadMessage()
    assert.Contains(t, string(data), "hello")
}
```

### Day 4 验收

```bash
go test ./pkg/hash/ -v                                # 一致性哈希
go test -tags=integration ./internal/ws/ -run "TestPubSub\|TestCrossNode" -v
# PASS: Redis Pub/Sub 送达
# PASS: A 在 Node1 → B 在 Node2 → 消息送达
```

---

## Day 5 测试计划：离线消息 + Kafka + E2E

**对应开发：** Day 5

| # | 测试内容 | 层级 | 环境 |
|---|---|---|---|
| T5.1 | B 不在线 → A 发消息 → 消息投递 Kafka topic `im:offline:user_b` | 集成 Medium | **真 Kafka** |
| T5.2 | B 上线 → Consumer 拉取 Kafka 离线消息 | 集成 Medium | **真 Kafka** |
| T5.3 | Consumer 推送后 → MySQL status=delivered | 集成 Medium | **真 MySQL** |
| T5.4 | Consumer 写 MySQL 失败 → 不 commit offset → 重试 | 集成 Medium | **真 Kafka** |
| T5.5 | 重复离线消息 → msg_id 幂等 → 不推送两次 | 集成 Medium | **真 MySQL UNIQUE** |
| T5.6 | Consumer Group 多实例 → 消息不重复消费 | 集成 Medium | **真 Kafka CG** |
| T5.7 | E2E：B 离线 → A 发 → B 上线 → Kafka → 收到 | E2E Large | 7 容器 |
| T5.8 | E2E：跨节点 + 离线组合 | E2E Large | 7 容器 |

### Day 5 核心用例

```go
func TestOfflineMessage_KafkaDelivery(t *testing.T) {
    // B 不在线
    // A 发送消息给 B
    sendMessage(t, connA, userB, "offline test")

    // 验证 Kafka topic 中有这条消息
    consumer := kafka.NewReader(kafka.ReaderConfig{
        Brokers: []string{"kafka:9092"},
        Topic:   "im:offline:user_b",
    })
    msg, err := consumer.ReadMessage(ctx)
    assert.NoError(t, err)
    assert.Contains(t, string(msg.Value), "offline test")

    // B 上线 → Consumer 拉取 → 推送到 B 的 WS → ACK → DB delivered
    connB := connectWS(t, "user-b")
    _, data, _ := connB.ReadMessage()
    assert.Contains(t, string(data), "offline test")
}
```

### Day 5 验收

```bash
go test -tags=integration ./internal/ws/ -run "TestOffline" -v
# PASS: Kafka offline message delivered

./scripts/e2e_test.sh
# ✅ 注册 + 登录 + 3节点分配
# ✅ 同节点消息 + ACK
# ✅ 跨节点消息（Redis Pub/Sub）
# ✅ 离线消息（Kafka）
```

---

## 测试统计

| 天 | 单元(零依赖) | 集成(真实Docker) | E2E(Docker全链路) | 累计 |
|---|---|---|---|---|
| Day 1 | 3 | 4 | 1 | 8 |
| Day 2 | 5 | 4 | 0 | 17 |
| Day 3 | 1 | 7 | 1 | 26 |
| Day 4 | 3 | 5 | 1 | 35 |
| Day 5 | 0 | 6 | 2 | **43** |

**比例：** 单元 12 (28%) / 集成 26 (60%) / E2E 5 (12%)

> 和之前版本的 mock 方案相比，集成测试从 21% 提升到 60%。代价是运行时间更长（需要 Docker 环境），但发现的都是真实问题。

---

## 测试环境隔离

```bash
# 测试专用 docker-compose.test.yml
# 每次测试前重建 MySQL + Redis（数据清空）
docker compose -f docker-compose.test.yml up -d
go test -tags=integration ./... -v
docker compose -f docker-compose.test.yml down -v
```

**测试数据隔离原则：**
- 每个集成测试 case 开始前 → TRUNCATE 相关表
- 测试间通过不同 user_id + msg_id 隔离
- E2E 测试用独立 docker compose 环境

---

## 红线（更新）

- ❌ **绝不 mock Redis Pub/Sub** — 这是跨节点消息路由的核心，必须真测
- ❌ **绝不 mock Kafka** — 这是离线消息的核心，必须真测
- ❌ **绝不 mock WebSocket** — gorilla/websocket 真客户端连接
- ✅ ConnManager / 一致性哈希 → 纯逻辑，不需要 mock
- ✅ 消息协议 JSON → 纯逻辑，不需要 mock
