# Feed 流系统 - 5 天测试计划

> 对标 `DEVELOPMENT_PLAN.md`，TDD 红-绿-重构循环。
> Feed 流的测试核心挑战：推拉结合状态一致性、多路归并正确性、并发扩散不丢数据。

---

## 测试金字塔分布

```
          ╱╲
         ╱  ╲         5% E2E (~5 条)
        ╱    ╲        发帖→关注→扩散→时间线→热门 全链路
       ╱──────╲
      ╱        ╲      15% 集成测试 (~10 条)
     ╱          ╲     MySQL repo + Redis timeline/outbox + 异步扩散Worker
    ╱────────────╲
   ╱              ╲   80% 单元测试 (~25 条)
  ╱                ╲  Snowflake、BigV判定、归并排序、发件箱裁剪、cursor分页、热度计算
 ╱──────────────────╲
```

### 测试策略原则

| 原则 | 应用 |
|------|------|
| **优先真实实现** | 集成测试用 Docker 里真实 MySQL + Redis，不走 mock |
| **单元测试小且快** | 纯逻辑（归并、分数计算、ID生成）用 fake/miniredis，毫秒级 |
| **DAMP 优于 DRY** | 每个测试自包含，不共享可变状态，不依赖执行顺序 |
| **状态断言** | 断言 Redis/MySQL 里的最终数据，不 mock 内部调用 |
| **禁止本地跑集成测试** | 集成测试和 E2E 必须走 Docker，不用 `127.0.0.1` |

---

## Day 1 测试计划：骨架 + 发帖 + 发件箱

**对应开发：** Day 1 — Snowflake ID、发帖 API、发件箱

| # | 测试内容 | 层级 | 环境 | 为什么这个层级 |
|---|---|---|---|---|
| T1.1 | Snowflake ID 全局唯一性：连续生成 1000 个 ID 无重复 | 单元 Small | 零依赖 | 纯算法，无 I/O |
| T1.2 | Snowflake ID 单调递增：后生成的 ID > 先生成的 ID | 单元 Small | 零依赖 | 纯算法，无 I/O |
| T1.3 | Snowflake ID 并发安全性：10 goroutine 各生成 100 个不冲突 | 单元 Small | 零依赖 | 纯并发，无外部依赖 |
| T1.4 | 创建帖子并写入 MySQL | 集成 Medium | **Docker MySQL** | 跨进程边界，需真实 DB |
| T1.5 | 创建帖子：content 为空返回错误 | 单元 Small | Fake repo | 参数校验，不用真实 DB |
| T1.6 | 发帖后写入发件箱 outbox:{user_id} | 集成 Medium | **Docker Redis** | Sorted Set 操作需真实 Redis |
| T1.7 | 发件箱上限裁剪：写入 600 条，保留最近 500 条 | 单元 Small | **miniredis** | Sorted Set 裁剪纯逻辑 |
| T1.8 | 查询帖子详情（PostID → Post） | 集成 Medium | **Docker MySQL** | 跨进程边界 |
| T1.9 | 查询不存在的帖子返回 nil | 集成 Medium | **Docker MySQL** | 边界条件 |

### Day 1 核心用例

```go
// T1.1: 唯一性
func TestSnowflake_Uniqueness(t *testing.T) {
    gen := idgen.NewSnowflake(1) // nodeID=1
    seen := make(map[int64]bool)
    for i := 0; i < 1000; i++ {
        id := gen.NextID()
        assert.False(t, seen[id], "duplicate ID: %d", id)
        seen[id] = true
    }
}

// T1.2: 单调递增
func TestSnowflake_Monotonic(t *testing.T) {
    gen := idgen.NewSnowflake(1)
    prev := gen.NextID()
    for i := 0; i < 100; i++ {
        curr := gen.NextID()
        assert.True(t, curr > prev, "%d should > %d", curr, prev)
        prev = curr
    }
}

// T1.3: 并发安全
func TestSnowflake_Concurrent(t *testing.T) {
    gen := idgen.NewSnowflake(1)
    ids := make(chan int64, 1000)
    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for j := 0; j < 100; j++ {
                ids <- gen.NextID()
            }
        }()
    }
    wg.Wait()
    close(ids)
    seen := make(map[int64]bool)
    for id := range ids {
        assert.False(t, seen[id], "concurrent duplicate: %d", id)
        seen[id] = true
    }
    assert.Equal(t, 1000, len(seen))
}

// T1.5: 空内容拒绝
func TestCreatePost_EmptyContent(t *testing.T) {
    svc := feed.NewService(fakeRepo{})
    _, err := svc.CreatePost(context.Background(), 1, "")
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "content")
}

// T1.7: 发件箱裁剪（用 miniredis）
func TestOutbox_TrimToLimit(t *testing.T) {
    mr := miniredis.RunT(t)
    rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

    outbox := feed.NewOutbox(rdb, 500) // 上限 500
    for i := 0; i < 600; i++ {
        outbox.Add(context.Background(), 1, int64(i), float64(i))
    }
    count := outbox.Count(context.Background(), 1)
    assert.Equal(t, 500, count, "should trim to 500")
}
```

### Day 1 验收

```bash
go test ./pkg/idgen/... -v              # T1.1-T1.3: 3 条
go test ./internal/feed/... -v -short   # T1.5, T1.7: 2 条 (unit)
go test ./internal/feed/... -v          # T1.4, T1.6, T1.8-T1.9: 4 条 (integration, 需 Docker)
# 预期: 9 PASS, 0 FAIL
```

---

## Day 2 测试计划：关注关系 + 写扩散（Push）

**对应开发：** Day 2 — 关注/取关 API、大V判定、写扩散

| # | 测试内容 | 层级 | 环境 | 为什么这个层级 |
|---|---|---|---|---|
| T2.1 | 关注用户 → MySQL + Redis 双写一致 | 集成 Medium | **Docker MySQL + Redis** | 双写原子性需验证真实存储 |
| T2.2 | 取关用户 → MySQL + Redis 双删一致 | 集成 Medium | **Docker MySQL + Redis** | 同上 |
| T2.3 | 重复关注幂等（不报错，不产生重复记录） | 集成 Medium | **Docker MySQL + Redis** | 幂等性需真实 DB 唯一约束 |
| T2.4 | 关注自己返回错误 | 单元 Small | Fake repo | 参数校验 |
| T2.5 | 大V判定：粉丝 >= 100000 → IsBigV = true | 单元 Small | Fake repo | 阈值比较纯逻辑 |
| T2.6 | 大V判定：粉丝 < 100000 → IsBigV = false | 单元 Small | Fake repo | 同上 |
| T2.7 | 大V判定边界：粉丝 = 99999 → false | 单元 Small | Fake repo | 边界条件 |
| T2.8 | 普通用户发帖 → 粉丝收件箱收到帖子 | 集成 Medium | **Docker MySQL + Redis** | 扩散正确性需真实 Redis |
| T2.9 | 大V 发帖 → 粉丝收件箱不变（仅写发件箱） | 集成 Medium | **Docker MySQL + Redis** | 推拉分叉核心逻辑 |
| T2.10 | 收件箱上限超 1000 条自动裁剪 | 单元 Small | **miniredis** | 裁剪算法纯逻辑 |
| T2.11 | 异步扩散：发帖立即返回，粉丝收件箱最终一致 | 集成 Medium | **Docker Redis + Worker** | 异步链路验证 |

### Day 2 核心用例

```go
// T2.5 - T2.7: 大V判定（纯逻辑，fake repo）
func TestIsBigV_AboveThreshold(t *testing.T) {
    checker := social.NewBigVChecker(fakeFollowerRepo{
        counts: map[int64]int{100: 100000},
    })
    assert.True(t, checker.IsBigV(100))
}

func TestIsBigV_BelowThreshold(t *testing.T) {
    checker := social.NewBigVChecker(fakeFollowerRepo{
        counts: map[int64]int{200: 99999},
    })
    assert.False(t, checker.IsBigV(200))
}

func TestIsBigV_ExactlyThreshold(t *testing.T) {
    checker := social.NewBigVChecker(fakeFollowerRepo{
        counts: map[int64]int{300: 99999},
    })
    assert.False(t, checker.IsBigV(300)) // 边界: < 100000

    checker2 := social.NewBigVChecker(fakeFollowerRepo{
        counts: map[int64]int{300: 100000},
    })
    assert.True(t, checker2.IsBigV(300)) // 边界: >= 100000
}

// T2.10: 收件箱裁剪（用 miniredis）
func TestTimeline_TrimToLimit(t *testing.T) {
    mr := miniredis.RunT(t)
    rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

    timeline := feed.NewTimeline(rdb, 1000)
    for i := 0; i < 1200; i++ {
        timeline.AddPost(context.Background(), 1, int64(i), float64(i))
    }
    count := timeline.Count(context.Background(), 1)
    assert.Equal(t, 1000, count)
    // 最小的时间戳（最老的帖子）应该被裁掉了
    oldest, _ := timeline.GetRange(context.Background(), 1, 0, 0)
    assert.Equal(t, int64(200), oldest[0]) // 保留最近 1000 条，即 200-1199
}

// T2.11: 异步扩散（需 Docker Redis + Worker）
func TestAsyncDiffusion_EventualConsistency(t *testing.T) {
    // 1. 用户1有3个粉丝
    // 2. 用户1发帖（非大V）
    // 3. 发帖接口立即返回（不阻塞）
    // 4. 等待 worker 处理完毕（最多 5s）
    // 5. 3个粉丝的 timeline 都收到帖子

    // 这是一个集成测试，需要真实 Redis + 运行中的 Worker
}
```

### Day 2 验收

```bash
go test ./internal/social/... -v -short  # T2.4-T2.7: 4 条 (unit)
go test ./internal/feed/... -v -short     # T2.10: 1 条 (unit, miniredis)
go test ./internal/social/... -v          # T2.1-T2.3: 3 条 (integration)
go test ./internal/feed/... -v            # T2.8, T2.9, T2.11: 3 条 (integration)
# 预期: 11 PASS (4 unit + 7 integration)
```

---

## Day 3 测试计划：时间线拉取 + 读扩散（Pull）

**对应开发：** Day 3 — Timeline API、多路归并、cursor 分页

| # | 测试内容 | 层级 | 环境 | 为什么这个层级 |
|---|---|---|---|---|
| T3.1 | 收件箱数据充足（>= limit）→ 直接返回 | 集成 Medium | **Docker Redis + 预置数据** | 需真实 Sorted Set 排序 |
| T3.2 | 收件箱不足 + 有关注大V → 触发 Pull 补充 | 集成 Medium | **Docker Redis** | Pull 路径关键链路 |
| T3.3 | K 路归并：2 路数据按时间倒序 merge | 单元 Small | 零依赖 | 纯算法 |
| T3.4 | K 路归并：5 路数据按时间倒序 merge | 单元 Small | 零依赖 | 纯算法 |
| T3.5 | K 路归并：空输入返回空 | 单元 Small | 零依赖 | 边界条件 |
| T3.6 | K 路归并：只有 1 路有数据 | 单元 Small | 零依赖 | 边界条件 |
| T3.7 | Cursor 分页：第一页返回 next_cursor + has_more | 集成 Medium | **Docker Redis** | 依赖真实数据排序 |
| T3.8 | Cursor 分页：最后一页 has_more = false | 集成 Medium | **Docker Redis** | 边界条件 |
| T3.9 | Cursor 分页：中间页数据不重复不遗漏 | 集成 Medium | **Docker Redis** | 分页一致性 |
| T3.10 | 无关注用户 → 时间线为空 | 集成 Medium | **Docker Redis** | 空状态处理 |

### Day 3 核心用例

```go
// T3.3 - T3.6: K 路归并（纯逻辑，零依赖）
type timelineItem struct {
    PostID    int64
    Timestamp float64
}

func TestMerge_TwoWaySorted(t *testing.T) {
    a := []timelineItem{
        {PostID: 1, Timestamp: 100},
        {PostID: 3, Timestamp: 80},
    }
    b := []timelineItem{
        {PostID: 2, Timestamp: 90},
        {PostID: 4, Timestamp: 70},
    }
    result := timeline.Merge(a, b)
    assert.Equal(t, 4, len(result))
    // 按时间倒序: 100, 90, 80, 70
    assert.Equal(t, int64(1), result[0].PostID) // ts=100
    assert.Equal(t, int64(2), result[1].PostID) // ts=90
    assert.Equal(t, int64(3), result[2].PostID) // ts=80
    assert.Equal(t, int64(4), result[3].PostID) // ts=70
}

func TestMerge_FiveWaySorted(t *testing.T) {
    // 5 路数据，验证归并后严格倒序
    lists := [][]timelineItem{
        {{PostID: 10, Timestamp: 100}},
        {{PostID: 20, Timestamp: 101}},
        {{PostID: 30, Timestamp: 99}},
        {{PostID: 40, Timestamp: 102}},
        {{PostID: 50, Timestamp: 98}},
    }
    result := timeline.MergeMulti(lists)
    times := make([]float64, len(result))
    for i, item := range result {
        times[i] = item.Timestamp
    }
    // 验证严格递减
    for i := 1; i < len(times); i++ {
        assert.True(t, times[i] <= times[i-1],
            "not sorted: times[%d]=%f > times[%d]=%f",
            i, times[i], i-1, times[i-1])
    }
    assert.Equal(t, int64(40), result[0].PostID) // ts=102, highest
    assert.Equal(t, int64(50), result[4].PostID) // ts=98, lowest
}

func TestMerge_Empty(t *testing.T) {
    result := timeline.Merge([]timelineItem{}, []timelineItem{})
    assert.Empty(t, result)
}

func TestMerge_OneSideEmpty(t *testing.T) {
    a := []timelineItem{{PostID: 1, Timestamp: 100}}
    result := timeline.Merge(a, []timelineItem{})
    assert.Equal(t, 1, len(result))
    assert.Equal(t, int64(1), result[0].PostID)
}

// T3.7-T3.9: Cursor 分页（需 Docker Redis 预置 30 条数据）
func TestCursorPagination_FirstPage(t *testing.T) {
    // 预置 30 条帖子到 timeline:1
    resp := GET("/api/v1/timeline?user_id=1&limit=10")
    assert.Equal(t, 0, resp.Code)
    assert.Equal(t, 10, len(resp.Data.Items))
    assert.NotEmpty(t, resp.Data.NextCursor)
    assert.True(t, resp.Data.HasMore)
}

func TestCursorPagination_LastPage(t *testing.T) {
    // 拉取到最后一页
    resp := GET("/api/v1/timeline?user_id=1&limit=10&cursor=" + lastCursor)
    assert.Equal(t, 0, resp.Code)
    assert.LessOrEqual(t, len(resp.Data.Items), 10)
    assert.False(t, resp.Data.HasMore)
}

func TestCursorPagination_NoDuplicates(t *testing.T) {
    seen := make(map[int64]bool)
    cursor := ""
    for {
        resp := GET("/api/v1/timeline?user_id=1&limit=10&cursor=" + cursor)
        for _, item := range resp.Data.Items {
            assert.False(t, seen[item.PostID],
                "duplicate post: %d", item.PostID)
            seen[item.PostID] = true
        }
        if !resp.Data.HasMore {
            break
        }
        cursor = resp.Data.NextCursor
    }
    assert.Equal(t, 30, len(seen)) // 总共 30 条，无重复无遗漏
}
```

### Day 3 验收

```bash
go test ./internal/timeline/... -v -short   # T3.3-T3.6: 4 条 (unit, 归并)
go test ./internal/timeline/... -v           # T3.1-T3.2, T3.7-T3.10: 6 条 (integration)
# 预期: 10 PASS (4 unit + 6 integration)
```

---

## Day 4 测试计划：热门 Feed + 新粉同步 + 容错

**对应开发：** Day 4 — 热度计算、关注同步、大V升降级、死信队列

| # | 测试内容 | 层级 | 环境 | 为什么这个层级 |
|---|---|---|---|---|
| T4.1 | 热度分值计算：新帖基础分正确 | 单元 Small | 零依赖 | 纯公式 |
| T4.2 | 热度分值计算：点赞加权 | 单元 Small | 零依赖 | 纯公式 |
| T4.3 | 热度分值计算：时间衰减（老帖分数降低） | 单元 Small | 零依赖 | 纯公式 |
| T4.4 | 热门 Feed 返回按热度降序 | 集成 Medium | **Docker Redis** | 需真实 ZSet 排序 |
| T4.5 | 关注非大V → 自动同步近 30 天帖子到新粉收件箱 | 集成 Medium | **Docker MySQL + Redis** | 跨存储同步验证 |
| T4.6 | 关注大V → 不同步历史帖子（Pull 模式自己拉） | 集成 Medium | **Docker MySQL + Redis** | 推拉分叉逻辑 |
| T4.7 | 取关 → 清理收件箱中该用户帖子 | 集成 Medium | **Docker Redis** | ZREM 操作 |
| T4.8 | 异步扩散重试 3 次后入死信队列 | 集成 Medium | **Docker Redis** | 容错链路 |
| T4.9 | 大V 升级（粉丝达 10万）→ 后续发帖走 Pull | 集成 Medium | **Docker MySQL + Redis** | 状态变更链 |
| T4.10 | 大V 降级（粉丝跌破 10万）→ 后续发帖走 Push | 集成 Medium | **Docker MySQL + Redis** | 状态变更链 |
| T4.11 | 限流中间件：超频返回 429 | 单元 Small | httptest | 中间件纯逻辑 |

### Day 4 核心用例

```go
// T4.1 - T4.3: 热度计算（纯逻辑）
func TestHotScore_NewPost(t *testing.T) {
    // 新帖基础分 = 当前时间戳 / 基准值
    now := time.Now().Unix()
    score := ranking.CalculateScore(0, 0, now)
    assert.Greater(t, score, 0.0)
}

func TestHotScore_LikesWeight(t *testing.T) {
    now := time.Now().Unix()
    scoreNoLike := ranking.CalculateScore(0, 0, now)
    scoreWithLike := ranking.CalculateScore(10, 0, now)
    assert.Greater(t, scoreWithLike, scoreNoLike)
}

func TestHotScore_TimeDecay(t *testing.T) {
    now := time.Now().Unix()
    scoreNew := ranking.CalculateScore(0, 0, now)
    scoreOld := ranking.CalculateScore(0, 0, now-86400*7) // 7 天前
    assert.Greater(t, scoreNew, scoreOld, "newer post should rank higher")
}

// T4.8: 死信队列
func TestRetryQueue_DeadLetter(t *testing.T) {
    // 构造一个必定失败的扩散任务
    // 重试 3 次后，确认任务进入 dead_letter:diffusion 队列
    mr := miniredis.RunT(t)
    rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

    worker := feed.NewDiffusionWorker(rdb, 3) // maxRetries=3
    // 注入一个必然会失败的 handler
    worker.Handle = func(task feed.DiffusionTask) error {
        return errors.New("always fail")
    }

    task := feed.DiffusionTask{PostID: 1, AuthorID: 1, Timestamp: 100}
    worker.Enqueue(context.Background(), task)
    worker.Process(context.Background())

    // 主队列应该为空
    qLen := rdb.LLen(context.Background(), "queue:diffusion").Val()
    assert.Equal(t, int64(0), qLen)

    // 死信队列应该有 1 条
    dlqLen := rdb.LLen(context.Background(), "dead_letter:diffusion").Val()
    assert.Equal(t, int64(1), dlqLen)
}

// T4.11: 限流中间件
func TestRateLimit_Exceeded(t *testing.T) {
    router := gin.New()
    limiter := middleware.NewRateLimiter(5, time.Minute) // 每分钟 5 次
    router.Use(limiter)
    router.GET("/test", func(c *gin.Context) { c.Status(200) })

    srv := httptest.NewServer(router)
    defer srv.Close()

    // 前 5 次成功
    for i := 0; i < 5; i++ {
        resp, _ := http.Get(srv.URL + "/test")
        assert.Equal(t, 200, resp.StatusCode, "request %d should pass", i+1)
    }

    // 第 6 次返回 429
    resp, _ := http.Get(srv.URL + "/test")
    assert.Equal(t, 429, resp.StatusCode)
}
```

### Day 4 验收

```bash
go test ./internal/ranking/... -v -short   # T4.1-T4.3: 3 条 (unit)
go test ./internal/middleware/... -v -short # T4.11: 1 条 (unit)
go test ./internal/feed/... -v -short       # T4.8: 1 条 (unit, miniredis)
go test ./internal/ranking/... -v           # T4.4: 1 条 (integration)
go test ./internal/social/... -v            # T4.5-T4.7: 3 条 (integration)
go test ./internal/feed/... -v              # T4.9-T4.10: 2 条 (integration)
# 预期: 11 PASS (5 unit + 6 integration)
```

---

## Day 5 测试计划：E2E + 并发 + 压测

**对应开发：** Day 5 — 全流程集成测试、性能验证

| # | 测试内容 | 层级 | 环境 | 验收标准 |
|---|---|---|---|---|
| T5.1 | 全链路 E2E：注册→关注→发帖→扩散→拉时间线→热门 | E2E Large | **Docker Compose 全栈** | 全流程 PASS，无报错 |
| T5.2 | 并发写扩散：50 用户同时发帖，200 粉丝收件箱完整 | E2E Large | **Docker Compose 全栈** | 200 个收件箱各包含 50 条 |
| T5.3 | 大V 读扩散：大V 发帖→粉丝 Pull 拉取 | E2E Large | **Docker Compose 全栈** | 粉丝时间线包含大V帖子 |
| T5.4 | 推拉边界：99999 粉丝走 Push，100000 粉丝走 Pull | E2E Large | **Docker Compose 全栈** | 扩散策略正确切换 |
| T5.5 | 时间线一致性：Push+Pull 混合场景，时间线按时间严格倒序 | E2E Large | **Docker Compose 全栈** | 两种来源 merge 后排序正确 |
| T5.6 | 性能基准：单机 QPS > 5000（发帖+拉时间线） | 压测 | **Docker Compose 全栈** | `wrk` 或 `go-wrk` 跑 |

### Day 5 核心用例

```go
// T5.1: 全链路 E2E（最重要的一条）
func TestE2E_FullFlow(t *testing.T) {
    baseURL := "http://feed:8080"

    // Step 1: 用户 1 关注用户 2
    resp := POST(baseURL + "/api/v1/follow", map[string]int64{
        "follower_id": 1, "followee_id": 2,
    })
    assert.Equal(t, 0, resp.Code)

    // Step 2: 用户 2 发帖（普通用户）
    resp2 := POST(baseURL + "/api/v1/posts", map[string]interface{}{
        "user_id": 2, "content": "E2E test post",
    })
    assert.Equal(t, 0, resp2.Code)
    postID := resp2.Data.PostID

    // Step 3: 等异步扩散完成
    time.Sleep(2 * time.Second)

    // Step 4: 用户 1 拉时间线，应该看到用户 2 的帖子
    tl := GET(baseURL + "/api/v1/timeline?user_id=1&limit=10")
    assert.Equal(t, 0, tl.Code)
    foundPost := false
    for _, item := range tl.Data.Items {
        if item.PostID == postID {
            foundPost = true
            break
        }
    }
    assert.True(t, foundPost, "follower should see followee's post in timeline")
}

// T5.2: 并发写扩散
func TestE2E_ConcurrentDiffusion(t *testing.T) {
    baseURL := "http://feed:8080"
    authorID := int64(100)
    fanCount := 200

    // 200 个粉丝关注作者
    for i := 0; i < fanCount; i++ {
        POST(baseURL + "/api/v1/follow", map[string]int64{
            "follower_id": int64(1000 + i),
            "followee_id": authorID,
        })
    }

    // 50 个帖子并发发出
    var wg sync.WaitGroup
    for i := 0; i < 50; i++ {
        wg.Add(1)
        go func(idx int) {
            defer wg.Done()
            POST(baseURL + "/api/v1/posts", map[string]interface{}{
                "user_id": authorID,
                "content": fmt.Sprintf("concurrent post %d", idx),
            })
        }(i)
    }
    wg.Wait()

    // 等异步扩散
    time.Sleep(5 * time.Second)

    // 每个粉丝的收件箱应该有 50 条帖子
    for i := 0; i < fanCount; i++ {
        tl := GET(baseURL + fmt.Sprintf(
            "/api/v1/timeline?user_id=%d&limit=100",
            1000+i,
        ))
        assert.Equal(t, 50, len(tl.Data.Items),
            "fan %d should have 50 posts, got %d",
            1000+i, len(tl.Data.Items))
    }
}

// T5.4: 推拉边界
func TestE2E_PushPullBoundary(t *testing.T) {
    baseURL := "http://feed:8080"

    // 场景 A: 用户 A 有 99999 粉丝 → 应走 Push
    // 发帖后，粉丝收件箱应该收到

    // 场景 B: 用户 B 有 100000 粉丝 → 应走 Pull
    // 发帖后，粉丝收件箱不变，但拉取时间线能通过 Pull 拿到
}
```

### Day 5 验收

```bash
# 启动全栈
docker-compose -f deployments/docker-compose.yaml up -d

# 等待就绪
sleep 10

# E2E 测试
go test ./test/e2e/... -v -count=1
# → T5.1 全链路
# → T5.2 并发扩散
# → T5.3 大V 读扩散
# → T5.4 推拉边界
# → T5.5 时间线一致性

# 性能压测
go test ./test/bench/... -v -bench=. -benchtime=10s
# → T5.6 发帖 QPS > 5000, 拉时间线 QPS > 5000

# 清理
docker-compose down -v
```

---

## 测试汇总

### 数量分布

| Day | 单元测试 (Small) | 集成测试 (Medium) | E2E (Large) | 合计 |
|-----|:---:|:---:|:---:|:---:|
| Day 1 | 5 | 4 | - | **9** |
| Day 2 | 5 | 6 | - | **11** |
| Day 3 | 4 | 6 | - | **10** |
| Day 4 | 5 | 6 | - | **11** |
| Day 5 | - | - | 5 + 压测 | **5** |
| **合计** | **19 (42%)** | **22 (48%)** | **5 (10%)** | **46** |

> 注：Feed 流系统的集成测试占比 (48%) 高于标准金字塔 (15%)，这是因为推拉结合的核心逻辑高度依赖 Redis Sorted Set 的排序和裁剪行为——这些行为用 mock 无法真实验证。我们的策略是：
> - **单元测试** 覆盖纯逻辑：归并排序、热度计算、ID 生成、参数校验、大V阈值
> - **集成测试** 覆盖有状态链路：Redis 扩散/裁剪/排序、MySQL 双写一致性
> - **E2E** 覆盖全流程和并发场景

### 环境依赖

| 测试类型 | 需要 | 启动方式 |
|---------|------|---------|
| 单元测试 (Small) | 无 / miniredis | 直接跑，无外部依赖 |
| 集成测试 (Medium) | Docker MySQL + Redis | `docker-compose -f deployments/docker-compose.yaml up -d mysql redis` |
| E2E (Large) | Docker Compose 全栈 | `docker-compose -f deployments/docker-compose.yaml up -d` |

### 运行指令

```bash
# 只跑单元测试（快，无需 Docker）
go test ./... -v -short

# 跑所有测试（需要 Docker）
docker-compose -f deployments/docker-compose.yaml up -d
go test ./... -v -count=1

# 只跑集成和 E2E
go test ./... -v -run "Integration|E2E" -count=1

# 带覆盖率
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out | grep total
```

---

## TDD 执行规则

每个 Day 的开发严格遵循红-绿-重构：

```
1. RED:   先写测试用例（参考上表），运行确认 FAIL
2. GREEN: 写最小实现代码让测试 PASS
3. REFACTOR: 重构代码，测试必须保持 PASS
4. 提交（原子提交，每个 PASS 一个 commit）
```

**禁止行为：**
- ❌ 先写代码后补测试
- ❌ 跳过 Day 测试直接开始下一个 Day
- ❌ 用 `t.Skip()` 绕过失败的测试
- ❌ 集成测试用 mock 替代真实 Redis/MySQL
- ❌ 测试中用 `127.0.0.1`（必须走 Docker 网络可路由地址）
