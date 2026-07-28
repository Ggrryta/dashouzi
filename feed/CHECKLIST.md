# Feed 流系统 —— 最终技术清单

> 20 个关键技术点，覆盖推拉结合架构全链路。每项含：问题 → 解法 → 代码位置 → 面试追问。

---

## 一、推拉结合架构（核心）

### 1. 写扩散 (Push)

**是什么：** 用户发帖时，将帖子 ID 写入所有粉丝的收件箱。

**为什么：** 读取时 O(1) 从收件箱取值，延迟低。适合粉丝量小的普通用户。

**代价：** 大V 发一条帖要写百万次 Redis → 写扩散风暴；存储放大 N 个粉丝 × M 条帖。

**代码位置：** `internal/feed/diffusion.go`

```go
func (d *Diffusion) Spread(authorID, postID, ts) {
    if d.follows.IsBigV(authorID) { return } // 大V不扩散
    followers := d.follows.GetFollowers(authorID)
    for _, fid := range followers {
        d.timeline.AddPost(fid, postID, ts)
    }
}
```

**面试追问：**
- Q: 写扩散的瓶颈在哪？→ A: 大V粉丝多时 Redis 写入 QPS 极高。限制最大扩散粉丝数 + 异步化
- Q: 如何优化？→ A: 异步扩散（发帖不阻塞）+ Pipeline 批量写入 + 限制最大粉丝数

---

### 2. 读扩散 (Pull)

**是什么：** 大V 只写自己的发件箱，粉丝拉取时间线时主动去拉大V发件箱。

**为什么：** 避免大V发帖写爆 Redis。读时归并，存储量 O(N+M) 而非 O(N×M)。

**代码位置：** `internal/timeline/service.go`

```go
// Step 1: 读自己的收件箱（Push 的内容）
inboxItems := readInbox(uid)

// Step 2: 不够 → Pull 大V 发件箱
if len(inboxItems) < limit {
    for _, fuid := range following {
        if isBigV(fuid) {
            allItems = append(allItems, readOutbox(fuid)...)
        }
    }
}
// Step 3: 去重 + 按时间倒序排序
```

**面试追问：**
- Q: 读扩散的缺点？→ A: 每次拉取都要遍历关注的大V，关注越多越慢
- Q: 优化方案？→ A: 热门大V缓存其发件箱快照；限制一次最多拉取的大V数量

---

### 3. 推拉阈值 10 万

**为什么是 10 万：** Twitter 选 5000，微博选 10000~50000。需平衡读写成本——太低则大部分走 Pull 增加读延迟，太高则大V写扩散成本高。10万是业界折中值。

**代码位置：** `cmd/feed/main.go` + `internal/social/service.go`

```go
socialSvc := social.NewService(socialRepo, outbox, 100000) // 阈值

func (c *BigVChecker) IsBigV(userID) bool {
    return c.repo.FollowerCount(userID) >= 100000
}
```

---

## 二、Redis 数据结构

### 4. Sorted Set 时间线

**为什么用 ZSet 而非 List：** score=时间戳 → 天然排序；支持 `ZREVRANGEBYSCORE` 游标分页； `ZREMRANGEBYRANK` 高效裁剪。

```
timeline:{user_id}  → ZSet(member=postID, score=timestamp)
outbox:{user_id}    → ZSet(member=postID, score=timestamp)
feed:hot            → ZSet(member=postID, score=hot_score)
```

**代码位置：** `internal/feed/timeline.go`、`internal/feed/outbox.go`、`internal/ranking/handler.go`

---

### 5. 收件箱上限裁剪

**问题：** 收件箱无限增长 → Redis 内存爆。

**解法：** ZAdd 后用 `ZREMRANGEBYRANK` 保留最近 N 条。

```go
pipe.ZAdd(key, redis.Z{Score: score, Member: postID})
pipe.ZRemRangeByRank(key, 0, -(maxLen + 1)) // 只保留最近 maxLen 条
```

**面试追问：**
- Q: 被裁剪的旧数据怎么办？→ A: 旧数据在 MySQL 持久化，冷数据回源查询
- Q: 1000 条上限够吗？→ A: 业务可配置，微信朋友圈大约 500-1000 条

---

### 6. 游标分页 (Cursor)

**为什么不用 offset：** `LIMIT 20 OFFSET 10000` → 扫描 10020 行丢弃 10000 行；新增数据导致分页漂移（重复/遗漏）。

**解法：** 时间戳游标 `ZREVRANGEBYSCORE MAX <cursor>`，从上一页最后一条继续。

```go
if cursor > 0 {
    results = ZRevRangeByScore(key, min="0", max=cursor-1, limit)
}
```

**代码位置：** `internal/timeline/service.go`、`internal/feed/timeline.go:GetRecent()`

---

## 三、存储设计

### 7. MySQL 持久化

**三张表：**

```sql
posts       → id, user_id, content, is_big_v(快照), created_at
follows     → follower_id, followee_id (联合主键, 天然去重)
big_v_users → user_id, follower_count, is_big_v, updated_at
```

**关键设计：**
- `posts.is_big_v` 是发帖时的快照 → 历史数据不受用户状态变更影响
- `follows` 联合主键 → `INSERT IGNORE` 幂等，天然防重复关注
- `big_v_users.follower_count` 事务原子更新，关注 +1 取关 -1

**代码位置：** `internal/feed/mysql_repo.go`、`internal/social/mysql_repo.go`

---

### 8. 双写一致性 / 事务安全

**关注流程：** MySQL 事务确保原子性。

```go
func (r *MySQLRepo) AddFollow(followerID, followeeID) {
    tx.Begin()
    tx.Exec("INSERT IGNORE INTO follows ...")
    tx.Exec("INSERT INTO big_v_users ... ON DUPLICATE KEY UPDATE follower_count+1")
    tx.Commit()
}
```

**面试追问：**
- Q: Redis 如何同步？→ A: 关注关系从 MySQL 直接查；时间线数据仅存 Redis
- Q: 扩散失败会丢帖吗？→ A: 发件箱在 MySQL + Redis 双存；扩散失败有重试 + 死信队列兜底

---

## 四、并发与异步

### 9. 异步扩散 + Goroutine

**为什么异步：** 粉丝多的用户发帖不能同步写 1000 个 Redis key，必须异步。

**模式：** `go func() { Spread() }()` — 进程内 goroutine，发帖立即返回。

**代码位置：** `internal/feed/diffusion.go:SpreadAsync()`

---

### 10. Snowflake 分布式 ID

**为什么不用自增 ID：** 分库分表后自增 ID 冲突。Snowflake 本地生成，不依赖 DB，高性能。

**结构：** `[41位时间戳] [10位机器ID] [12位序列号]` = 64位

```go
func (s *Snowflake) NextID() int64 {
    now := time.Now().UnixMilli()
    return ((now - epoch) << timeShift) | (nodeID << nodeShift) | sequence
}
```

**代码位置：** `pkg/idgen/snowflake.go`

---

## 五、归并与去重

### 11. K 路归并

**场景：** Push 收件箱 + N 个大V 发件箱 = K 路已排序列表，合并为一个有序列表。

**算法：** 两路归并 O(N×K)，K<100 时比最小堆更简单高效。

```go
func Merge(a, b []Item) []Item {
    for i < len(a) && j < len(b) {
        if a[i].Timestamp >= b[j].Timestamp { result = append(a[i]); i++ }
        else { result = append(b[j]); j++ }
    }
}
func MergeMulti(lists [][]Item) []Item { // K路递归归并 }
```

**代码位置：** `internal/timeline/merge.go`

**面试追问：**
- Q: K 很大时优化？→ A: 最小堆 O(N log K)
- Q: 内存问题？→ A: 只取 Top N（通常 20），其余丢弃

---

### 12. 去重策略

**为什么需要：** 同一个大V的帖子可能同时出现在 Push 收件箱（历史数据）和 Pull 发件箱。

**解法：** 归并后用 `map[postID]bool` 去重。

**代码位置：** `internal/timeline/service.go:deduplicate()`

---

## 六、热门 Feed

### 13. HackerNews 热度算法

**公式：** `score = (likes × 2 + comments × 1 + 1) / (hours_since_post + 2)^1.5`

**设计要点：**
- 点赞权重 > 评论权重（2:1）
- 分母指数衰减 → 老帖分数快速下降
- `+1` 底数 → 新帖即使 0 赞也有基础分
- `+2` 防除零 → 0 小时帖子不无穷大

**代码位置：** `internal/ranking/score.go`

```go
func CalculateScore(likes, comments int, postTime int64) float64 {
    hoursSince := float64(time.Now().Unix()-postTime) / 3600.0
    numerator := float64(likes*2 + comments + 1)
    denominator := math.Pow(hoursSince+2, 1.5)
    return numerator / denominator
}
```

**面试追问：**
- Q: 和 Reddit 算法的区别？→ A: Reddit 用 Wilson 置信区间，关注"争议度"；HackerNews 简单高效
- Q: 热门 Feed 多久更新一次？→ A: 定时任务（比如每 5 分钟）批量更新 Redis ZSet

---

## 七、容错与可观测

### 14. 异步扩散重试 + 死信队列

**场景：** 异步扩散可能因 Redis 抖动失败，不能直接丢弃。

**策略：** Redis List 做队列，失败重试 3 次（退避等待 100ms/200ms/300ms），仍失败入死信队列（`dead_letter:diffusion`），后续可人工或定时任务处理。

```go
func (w *RetryWorker) ProcessOne(ctx, handler) {
    for attempt := 0; attempt <= maxRetry; attempt++ {
        if err := handler(task); err == nil { return }
        time.Sleep(backoff(attempt))       // 退避重试
    }
    w.rdb.LPush(dlqKey, data)            // 入死信队列
}
```

**代码位置：** `internal/feed/retry.go`

---

### 15. 新粉历史同步

**场景：** 关注一个普通用户 → 应看到被关注者近期帖子。

**处理：** 关注非大V → 异步拉取被关注者发件箱 → 批量写入新粉收件箱。大V 不走同步（下次 Pull 自然拉取）。

```go
func (s *Service) Follow(followerID, followeeID) {
    s.repo.AddFollow(...)                         // 1. 写入关注关系
    if !s.bigV.IsBigV(followeeID) {              // 2. 非大V才同步
        go s.syncRecentPosts(followerID, followeeID)
    }
    s.checkBigVUpgrade(followeeID)                // 3. 检测大V升级
}
```

**代码位置：** `internal/social/service.go:Follow()` + `syncRecentPosts()`

---

### 16. 取关清理收件箱

**场景：** 取关后不应再看到该用户的帖子。

**处理：** 取关 → 查被取关者发件箱 → `ZREM` 批量移除对应帖子。

```go
func (s *Service) Unfollow(followerID, followeeID) {
    s.repo.RemoveFollow(...)
    s.cleaner.RemoveUserPosts(followerID, followeeID) // 清理收件箱
    s.checkBigVDowngrade(followeeID)                   // 检测大V降级
}
```

**代码位置：** `internal/social/service.go:Unfollow()`

---

### 17. 大V 升降级自动切换

**场景：** 用户粉丝跨过 10 万阈值 → 发帖策略需要切换。

**升级（普通→大V）：** 粉丝达 10万 → `SetBigV(userID, true)` → 后续发帖走 Pull，不扩散
**降级（大V→普通）：** 粉丝跌破 10万 → `SetBigV(userID, false)` → 下次发帖自动切回 Push

**代码位置：** `internal/social/service.go:checkBigVUpgrade()` + `checkBigVDowngrade()`

**面试追问：**
- Q: 切换瞬间的历史数据怎么办？→ A: 历史帖已在粉丝收件箱，不影响；新帖按当前状态决定
- Q: 频繁跨越阈值怎么办？→ A: 可加滞后区间（如 9.5万降级，10.5万升级）

---

## 八、安全

### 18. 固定窗口限流

**策略：** 按用户 ID（`X-User-ID` header）或 IP 做固定窗口计数，超频返回 429。

```go
type RateLimiter struct {
    mu       sync.Mutex
    windows  map[string]*rateWindow  // key → {count, resetAt}
    limit    int                      // 每窗口上限
    interval time.Duration            // 窗口大小
}
```

**代码位置：** `internal/middleware/ratelimit.go`

**面试追问：**
- Q: 固定窗口的临界问题？→ A: 窗口边界可能双倍流量冲击；生产可用滑动窗口或 Token Bucket
- Q: 分布式限流怎么做？→ A: Redis INCR + EXPIRE，或 Redis Lua 脚本原子化

---

## 九、架构决策

### 19. 发帖时快照 is_big_v

**为什么不在 timeline 查询时 JOIN：** 用户可能从普通用户变成大V，如果 JOIN 实时查询，历史帖的分发策略会随状态变化——已扩散到粉丝收件箱的帖子可能被错误地当作"大V帖子"二次处理。

**解法：** `posts.is_big_v` 存储发帖时的状态快照，不随用户状态变化。

**代码位置：** `internal/feed/mysql_repo.go` POSTS 表 DDL

---

### 20. 接口适配器模式（Adapter Pattern）

**为什么用适配器：** Feed、Social、Timeline 三个领域各自定义了接口（`FollowerProvider`、`TimelineCleaner`、`SyncWriter` 等），底层实现可以互换。通过适配器解耦，便于单元测试注入 fake、未来替换存储。

```go
// FollowerProvider 是 feed 包定义的接口
// social.MySQLRepo 是 social 包的实现
// socialAdapter 桥接两者
type socialAdapter struct { repo *social.MySQLRepo }
func (a *socialAdapter) GetFollowers(...) { return a.repo.GetFollowers(...) }
```

**代码位置：**
- `cmd/feed/main.go:socialAdapter` — `social.MySQLRepo` → `feed.FollowerProvider`
- `internal/timeline/adapter.go` — 5 个适配器

---

## 完整技术栈

| 层 | 技术 | 作用 |
|----|------|------|
| HTTP | Gin | 路由 + 中间件 |
| ID | Snowflake | 全局唯一分布式 ID |
| 缓存 | Redis ZSet / List | 收件箱、发件箱、热门排行、死信队列 |
| 持久化 | MySQL 8.0 | 帖子、关注关系、大V表 |
| 扩散 | Goroutine 异步 | 写扩散到粉丝收件箱 |
| 容错 | 退避重试 + 死信队列 | 异步扩散失败兜底 |
| 归并 | K 路归并算法 | Push + Pull 数据合并 |
| 热度 | HackerNews 公式 | 热门 Feed 排行 |
| 分页 | Cursor 时间戳游标 | 高性能翻页，无漂移 |
| 限流 | 固定窗口 | 防刷保护 |
| 测试 | miniredis + testify | 29 单元测试 + 5 E2E |
| 部署 | Docker Compose | 三服务一键编排 |

---

## 面试高频追问速查

| 问题 | 速答 |
|------|------|
| 推拉结合怎么选阈值？ | 粉丝 < 10万走 Push，>= 10万走 Pull，平衡读写成本 |
| 大V 发帖怎么保证不丢？ | 发件箱 Redis + MySQL 双存；粉丝 Pull 时拉取 |
| Redis 宕机了怎么办？ | 收件箱可回源 MySQL 重建；发件箱=MySQL posts 表替代 |
| 游标分页 vs offset？ | Cursor 基于时间戳，无深分页；新增数据不影响已翻页面 |
| 为什么用 ZSet 不用 List？ | ZSet 天然排序 + 范围查询 + 排名裁剪 |
| 关注/取关原子性？ | MySQL 事务 + INSERT IGNORE 幂等 + follower_count 原子加减 |
| 并发扩散会丢吗？ | 异步 goroutine + 重试 3 次 + 死信队列兜底 |
| 新关注用户能看到历史帖吗？ | 关注非大V自动同步近期帖子；关注大V下次 Pull 自然拉取 |
| 大V 掉粉降级后发帖策略？ | 自动检测粉丝数 → 跌破阈值 → 切回 Push 扩散 |
| HackerNews 热度公式细节？ | `(likes×2+comments+1) / (hours+2)^1.5`，点赞权重 2:1 |
| 限流窗口临界问题？ | 固定窗口有边界双倍问题；生产用 Token Bucket 或滑动窗口 |
| 接口设计的适配器模式？ | Feed/Social/Timeline 各自定义接口，适配器解耦、便于测试和替换 |
