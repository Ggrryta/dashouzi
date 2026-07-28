# Feed 流系统 - 5 天开发排期计划

> 每天 4-6 小时。垂直切片：每天交付一个可验证的功能模块。
> Feed 流是社交核心系统，重点在推拉结合架构的正确实现与性能优化。

---

## Day 1：骨架 + 发帖 + 发件箱

**目标：** 创建帖子并写入发件箱，为后续扩散做准备。

### 上午（3h）

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 1.1 | `go mod init feed`，Gin 空壳 | `cmd/feed/main.go` | `/health` 返回 OK |
| 1.2 | MySQL 建表 + 连接 | `internal/feed/repo.go` | posts 表创建，Insert/Query 通过 |
| 1.3 | Redis 连接 + 基础操作 | `pkg/redis/` | Sorted Set 读写通过 |
| 1.4 | 雪花 ID 生成器 | `pkg/idgen/snowflake.go` | 全局唯一递增 ID |

### 下午（3h）

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 1.5 | 发帖 API Handler | `internal/feed/handler.go` | POST `/api/v1/posts` 创建帖子 |
| 1.6 | 写入发件箱 outbox:{uid} | `internal/feed/outbox.go` | 发帖后 outbox ZSet 包含帖子ID |
| 1.7 | 发件箱上限裁剪（保留最近 500 条） | outbox 内 | ZREMRANGEBYRANK 自动裁剪 |
| 1.8 | Dockerfile + docker-compose（app + MySQL + Redis） | 容器编排 | `curl POST 发帖` 返回 post_id |
| 1.9 | Day 1 单元测试 | `*_test.go` | 发帖+发件箱 测试通过 |

### Day 1 验收

```bash
# 发帖
curl -X POST http://localhost:8080/api/v1/posts \
  -H "Content-Type: application/json" \
  -d '{"user_id":1,"content":"Hello Feed!"}'
# → {"code":0,"data":{"post_id":123456,"created_at":"..."}}

# 验证发件箱
redis-cli ZRANGE outbox:1 0 -1 WITHSCORES
# → 帖子ID 和 时间戳
```

---

## Day 2：关注关系 + 写扩散（Push）

**目标：** 建立关注关系，普通用户发帖时自动扩散到粉丝收件箱。

### 上午（3h）

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 2.1 | 关注/取关 API | `internal/social/handler.go` | POST follow/unfollow 成功 |
| 2.2 | 关注关系 MySQL + Redis 双写 | `internal/social/repo.go` | follows 表 + following Set 同步 |
| 2.3 | 粉丝列表查询 | `internal/social/follower.go` | 获取某用户全部粉丝 ID |
| 2.4 | 大V 判定（阈值 10万） | `internal/social/bigv.go` | IsBigV(uid) 返回 true/false |

### 下午（3h）

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 2.5 | 写扩散逻辑：发帖 → 查粉丝 → 写收件箱 | `internal/feed/diffusion.go` | 非大V发帖，粉丝收到 timeline |
| 2.6 | 异步扩散（Redis List 队列 + Worker） | `internal/feed/worker.go` | 发帖不阻塞，异步写入粉丝 |
| 2.7 | 收件箱上限裁剪（保留最近 1000 条） | diffusion 内 | timeline ZSet 不超 1000 |
| 2.8 | 大V 发帖不做扩散，仅写发件箱 | diffusion 内 | 大V发帖后粉丝 timeline 不变 |
| 2.9 | Day 2 单元测试 | `*_test.go` | 关注+扩散 测试通过 |

### Day 2 验收

```bash
# 用户 2 关注用户 1
curl -X POST http://localhost:8080/api/v1/follow \
  -d '{"follower_id":2,"followee_id":1}'

# 用户 1 发帖（普通用户）
curl -X POST http://localhost:8080/api/v1/posts \
  -d '{"user_id":1,"content":"Push test"}'

# 验证用户 2 的收件箱收到帖子
redis-cli ZRANGE timeline:2 0 -1
# → 包含帖子 ID
```

---

## Day 3：时间线拉取 + 读扩散（Pull）

**目标：** 拉取个人时间线，混合收件箱（Push内容）和 大V发件箱（Pull内容）。

### 上午（3h）

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 3.1 | GET Timeline API | `internal/timeline/handler.go` | cursor 分页返回时间线 |
| 3.2 | 从收件箱读取 Push 内容 | `internal/timeline/inbox.go` | ZREVRANGEBYSCORE 按时间倒序 |
| 3.3 | 计算收件箱数据缺口 | timeline 内 | 收件箱不足 limit 时触发 Pull |
| 3.4 | 查询关注列表中大V的发件箱 | `internal/timeline/pull.go` | 逐个拉取大V outbox |

### 下午（3h）

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 3.5 | Push + Pull 多路归并排序 | `internal/timeline/merge.go` | K 路归并，按时间倒序取 Top N |
| 3.6 | 帖子详情批量填充 | `internal/timeline/fill.go` | 返回完整帖子内容+作者信息 |
| 3.7 | cursor 分页实现 | timeline 内 | next_cursor + has_more |
| 3.8 | 冷数据回源 MySQL | timeline 内 | Redis 无数据时查 MySQL |
| 3.9 | Day 3 单元测试 | `*_test.go` | 时间线拉取+归并 测试通过 |

### Day 3 验收

```bash
# 拉取用户 2 的时间线
curl "http://localhost:8080/api/v1/timeline?user_id=2&limit=20"
# → {"code":0,"data":{"items":[...],"next_cursor":"...","has_more":false}}

# 场景：关注 1 个普通用户 + 1 个大V
# 时间线应包含两者的最新帖子，按时间排序
```

---

## Day 4：热门 Feed + 新粉同步 + 容错

**目标：** 热门 Feed 排行、新关注时的历史同步、异常处理。

### 上午（3h）

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 4.1 | 热门 Feed 排行（按时间+互动加权） | `internal/ranking/hot.go` | GET `/api/v1/feed/hot` 返回热门 |
| 4.2 | 热度分值计算 | `internal/ranking/score.go` | 类似 HackerNews 算法 |
| 4.3 | 新关注同步近 30 天帖子 | `internal/social/sync.go` | 关注非大V时拉历史入收件箱 |
| 4.4 | 取关清理收件箱 | social 内 | ZREM 移除取关用户帖子 |

### 下午（3h）

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 4.5 | 异步扩散失败重试 + 死信队列 | `internal/feed/retry.go` | 重试3次后入死信 |
| 4.6 | 大V 状态变更处理（普通→大V） | social 内 | 后续发帖改为 Pull 模式 |
| 4.7 | 大V 降级处理（大V→普通） | social 内 | 降级后发帖改为 Push 模式 |
| 4.8 | 限流中间件 | `internal/middleware/ratelimit.go` | 单用户每分钟 30 次 |
| 4.9 | Day 4 单元测试 | `*_test.go` | 热门+同步+容错 测试通过 |

### Day 4 验收

```bash
# 热门 Feed
curl "http://localhost:8080/api/v1/feed/hot?limit=20"

# 新粉关注非大V，自动同步历史帖子
# 关注后，新粉 timeline 包含被关注者近30天帖子
```

---

## Day 5：集成测试 + Docker 联调 + 文档

**目标：** 全流程 Docker Compose 集成测试，编写技术清单。

### 上午（3h）

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 5.1 | E2E 测试脚本 | `test/e2e/main_test.go` | 发帖→关注→扩散→拉取全流程 |
| 5.2 | 并发写扩散测试 | test 内 | 100 用户同时发帖，扩散正确 |
| 5.3 | 大V 读扩散正确性测试 | test 内 | 大V发帖后粉丝拉取正确 |
| 5.4 | 推拉阈值边界测试 | test 内 | 9.9万粉丝Push，10万粉丝Pull |

### 下午（3h）

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 5.5 | docker-compose 最终编排 | `deployments/docker-compose.yaml` | 一键启动全服务 |
| 5.6 | 性能压测脚本 | `test/bench/` | 单机 QPS > 5000 |
| 5.7 | Feed 系统关键技术清单 | `CHECKLIST.md` | 推拉结合 15+ 技术点 |
| 5.8 | API 文档 | `api/README.md` | 完整 API 说明 |

### Day 5 验收

```bash
# 全流程自动化测试
docker-compose up -d
go test ./test/e2e/... -v
# → PASS: 发帖→关注→扩散→时间线→热门 全部通过
```

---

## 每日交付物总览

| Day | 核心功能 | 测试数目标 | 关键文件 |
|-----|---------|-----------|---------|
| 1 | 发帖 + 发件箱 | 6+ | feed/handler.go, feed/outbox.go |
| 2 | 关注 + 写扩散 | 8+ | social/handler.go, feed/diffusion.go |
| 3 | 时间线 + 读扩散 | 8+ | timeline/handler.go, timeline/merge.go |
| 4 | 热门 + 同步 + 容错 | 8+ | ranking/hot.go, social/sync.go |
| 5 | 集成测试 + 文档 | E2E | test/e2e/, CHECKLIST.md |

**总测试数目标：30+，覆盖推拉结合全部核心链路。**
