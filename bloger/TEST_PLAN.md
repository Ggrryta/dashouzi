# 博客系统 - 7 天测试计划

> 对标 `DEVELOPMENT_PLAN.md`，采用 **TDD 红-绿-重构** 循环。
> 测试金字塔：**80% 单元测试 / 15% 集成测试 / 5% E2E 测试**。
> 所有测试在 Docker 内运行。

---

## 测试金字塔分布

```
          ╱╲
         ╱  ╲         5% E2E 测试 (~10 条)
        ╱    ╲        完整用户流程，真实 Docker 环境
       ╱──────╲
      ╱        ╲      15% 集成测试 (~25 条)
     ╱          ╲     API 边界、中间件链、DB 交互
    ╱────────────╲
   ╱              ╲   80% 单元测试 (~130 条)
  ╱                ╲  纯逻辑、隔离、毫秒级
 ╱──────────────────╲
```

| 测试层级 | 数量 | 运行时间 | 位置 | 测试对象 |
|---|---|---|---|---|
| **Small (单元)** | ~130 | < 5s | `*_test.go` 同目录 | Service、工具函数、纯逻辑 |
| **Medium (集成)** | ~25 | < 30s | `internal/..._integration_test.go` | Handler + DB、中间件链、API |
| **Large (E2E)** | ~10 | < 2min | `scripts/integration_test.sh` | Docker 内全链路 |

---

## 总则：TDD 工作流

每个功能的开发节奏：

```
  RED                    GREEN                  REFACTOR
写失败的测试  ──────→  写最小代码让测试通过  ──────→  重构代码，测试保持绿
     │                       │                         │
     ▼                       ▼                         ▼
 测试 FAILS              测试 PASSES              测试仍然 PASSES
```

**关键规则：**
1. **先写测试，再写代码**。写代码前测试必须红（失败）。
2. **最小实现**。只写让测试通过的代码，不过度设计。
3. **重构时不停跑测试**。每步重构后立即跑一次。
4. **DAMP 优于 DRY**。测试代码允许重复，每个测试自描述。
5. **断言结果而非实现**。测输出不测内部调用。
6. **每个测试只验一个概念**。一个测试只测一件事。

---

## Day 1 测试计划：项目骨架 + 基础设施

**对应开发：** Day 1 — Go 项目初始化、Docker 环境、中间件骨架

| # | 测试内容 | 层级 | 大小 | 对应开发任务 | RED 条件 |
|---|---|---|---|---|---|
| T1.1 | 配置加载：从 yaml + 环境变量正确读取 | 单元 | Small | 1.2 | config 包不存在 |
| T1.2 | 统一响应 JSON 格式正确 | 单元 | Small | 1.4 | response 包不存在 |
| T1.3 | 错误码定义：每个错误码 message 非空 | 单元 | Small | 1.4 | errcode 包不存在 |
| T1.4 | GORM 连接 PostgreSQL 并自动建表 | 集成 | Medium | 1.5 | 表不存在 |
| T1.5 | Docker Compose 三服务健康检查 | E2E | Large | 1.7 | 服务未启动 |
| T1.6 | Recovery 中间件：panic 后返回 500 而非崩溃 | 集成 | Medium | 1.8 | 中间件不存在 |
| T1.7 | Logger 中间件：每个请求输出结构化日志 | 集成 | Medium | 1.8 | 中间件不存在 |
| T1.8 | `/api/v1/ping` 返回 `{"code":0,"message":"ok","data":"pong"}` | 集成 | Medium | 1.1 | 路由未注册 |

### Day 1 测试用例详述

```go
// T1.1: config_test.go — 配置加载
func TestLoadConfig_FromYAML(t *testing.T) {
    // 验证: 从 config.yaml 读取到 server.port, db.host, db.port 等
}
func TestLoadConfig_EnvOverride(t *testing.T) {
    // 验证: 环境变量 DB_HOST=10.0.0.1 覆盖 yaml 中的值
}

// T1.2: response_test.go — 统一响应
func TestSuccess_ReturnsCodeZero(t *testing.T) {
    // 验证: response.Success(c, data) → {"code":0,"message":"ok","data":...}
}
func TestError_ReturnsCorrectCode(t *testing.T) {
    // 验证: response.Error(c, errcode.ErrNotFound) → code 非 0

// T1.3: errcode_test.go — 错误码
func TestErrCode_Unique(t *testing.T) {
    // 验证: 所有已定义错误码互不重复
}
func TestErrCode_HasMessage(t *testing.T) {
    // 验证: 每个错误码的 Message 字段非空
}

// T1.6: recovery_test.go — Recovery 中间件
func TestRecovery_Returns500_OnPanic(t *testing.T) {
    // Arrange: 注册一个故意 panic 的路由
    // Act: GET /panic
    // Assert: HTTP 500, body 包含 {code: 50000}
}

// T1.7: logger_test.go — Logger 中间件
func TestLogger_OutputsRequestInfo(t *testing.T) {
    // Act: 发一个 GET 请求
    // Assert: 日志输出包含 method, path, status, latency
}
```

### Day 1 验收标准

```bash
# 1. 单元测试全绿
go test ./pkg/... -v
# ok  bloger/pkg/config      0.1s
# ok  bloger/pkg/response    0.1s
# ok  bloger/pkg/errcode     0.1s

# 2. 集成测试（Docker 内）
docker compose up -d
go test ./internal/middleware/... -v -tags=integration
# ok  bloger/internal/middleware  2.1s

# 3. E2E 冒烟测试
curl http://<real-ip>:8080/api/v1/ping
# {"code":0,"message":"ok","data":"pong"}
```

**Day 1 RED 标志：** 写任何实现代码前，所有测试全部失败。

---

## Day 2 测试计划：用户模块 — 注册/登录/JWT

**对应开发：** Day 2 — User Model、Repository、Service、Handler、Auth/Role 中间件

| # | 测试内容 | 层级 | 大小 | TDD 顺序 | RED 条件 |
|---|---|---|---|---|---|
| T2.1 | 密码 bcrypt 加密：明文存入后为 hash，且可验证 | 单元 | Small | 先于 2.2 | BeforeCreate 钩子不存在 |
| T2.2 | UserRepo.Create：成功创建用户 | 集成 | Medium | 先于 2.3 | repo 包为空 |
| T2.3 | UserRepo.FindByEmail：查到存在/不存在的用户 | 集成 | Medium | 先于 2.3 | repo 包为空 |
| T2.4 | UserRepo 唯一性冲突：同 email/username 插入报错 | 集成 | Medium | 先于 2.3 | — |
| T2.5 | Register：成功注册，返回用户信息（不含密码） | 单元 | Small | 先于 2.4 | service 包为空 |
| T2.6 | Register：邮箱已存在返回 ErrEmailExists | 单元 | Small | 先于 2.4 | — |
| T2.7 | Register：用户名为空/过短/含非法字符校验 | 单元 | Small | 先于 2.4 | — |
| T2.8 | Login：正确密码返回 token | 单元 | Small | 先于 2.4 | — |
| T2.9 | Login：错误密码返回 ErrInvalidPassword | 单元 | Small | 先于 2.4 | — |
| T2.10 | JWT GenerateToken：生成合法 token | 单元 | Small | 先于 2.5 | jwt 包为空 |
| T2.11 | JWT ParseToken：解析 token 得到 user_id + role | 单元 | Small | 先于 2.5 | — |
| T2.12 | JWT ParseToken：过期 token 返回错误 | 单元 | Small | 先于 2.5 | — |
| T2.13 | JWT ParseToken：篡改 token 返回错误 | 单元 | Small | 先于 2.5 | — |
| T2.14 | Auth 中间件：无 token → 401 | 集成 | Medium | 先于 2.8 | 中间件不存在 |
| T2.15 | Auth 中间件：有效 token → 放行，ctx 中有 user_id | 集成 | Medium | 先于 2.8 | — |
| T2.16 | Auth 中间件：过期 token → 401 | 集成 | Medium | 先于 2.8 | — |
| T2.17 | Role 中间件：reader 访问 author 接口 → 403 | 集成 | Medium | 先于 2.9 | — |
| T2.18 | Role 中间件：admin 访问任何接口 → 放行 | 集成 | Medium | 先于 2.9 | — |
| T2.19 | POST /users/register：端到端注册成功 | E2E | Large | 最后 | 路由未注册 |
| T2.20 | POST /users/login：端到端登录拿 token | E2E | Large | 最后 | — |
| T2.21 | GET /users/me：带 token 获取个人信息 | E2E | Large | 最后 | — |

### Day 2 测试用例详述

```go
// T2.5: user_test.go — Register 成功
func TestRegister_Success(t *testing.T) {
    // Arrange: mock repo 返回 nil（无重复）
    // Act: service.Register(ctx, {username:"test", email:"t@t.com", password:"123456"})
    // Assert: 返回 User{Username:"test", Email:"t@t.com"}
    //         PasswordHash 不是 "123456"（已被加密）
    //         PasswordHash 可被 bcrypt 验证
}

// T2.8: user_test.go — Login 成功
func TestLogin_Success(t *testing.T) {
    // Arrange: mock repo FindByEmail 返回用户，密码 hash 是 "123456" 的 bcrypt
    // Act: service.Login(ctx, "t@t.com", "123456")
    // Assert: 返回的 token 非空，ParseToken 后 user_id 匹配
}

// T2.14: auth_test.go — 无 token
func TestAuth_MissingToken_Returns401(t *testing.T) {
    // Arrange: 创建无 Authorization header 的请求
    // Act: 经过 auth 中间件
    // Assert: c.Abort() 被调用，响应码 401
}

// T2.17: role_test.go — reader 无权限
func TestRole_Reader_NoAccess_ToAuthorEndpoint(t *testing.T) {
    // Arrange: ctx 中 user.Role = "reader"，目标路由需要 "author"
    // Act: 经过 role 中间件
    // Assert: 返回 403
}
```

### Day 2 验收标准

```bash
# 单元测试
go test ./internal/service/... ./pkg/jwt/... -v
# ok  bloger/internal/service  0.5s  coverage: 85%
# ok  bloger/pkg/jwt           0.2s  coverage: 90%

# 集成测试
go test ./internal/middleware/... ./internal/repository/... -v -tags=integration
# ok  bloger/internal/middleware    2.0s
# ok  bloger/internal/repository   1.5s

# E2E（Docker 内）
curl -X POST http://<real-ip>:8080/api/v1/users/register \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","email":"admin@test.com","password":"Test1234"}'
# {"code":0,"message":"ok","data":{"id":1,"username":"admin","email":"admin@test.com",...}}

curl -X POST http://<real-ip>:8080/api/v1/users/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@test.com","password":"Test1234"}'
# {"code":0,"message":"ok","data":{"token":"eyJ..."}}

# 无 token 访问 → 401
curl http://<real-ip>:8080/api/v1/users/me
# {"code":40100,"message":"missing authorization header","data":null}
```

---

## Day 3 测试计划：文章模块 — CRUD + 状态机 + 标签

**对应开发：** Day 3 — Article/Tag Model、Repository、Service、Handler

| # | 测试内容 | 层级 | 大小 | TDD 顺序 | RED 条件 |
|---|---|---|---|---|---|
| T3.1 | Article 状态机：draft → reviewing | 单元 | Small | 先于 3.4 | 状态机函数不存在 |
| T3.2 | 状态机：reviewing → published | 单元 | Small | 先于 3.4 | — |
| T3.3 | 状态机：published → archived | 单元 | Small | 先于 3.4 | — |
| T3.4 | 状态机：reviewing → draft (驳回) | 单元 | Small | 先于 3.4 | — |
| T3.5 | 状态机：archived → draft (重新编辑) | 单元 | Small | 先于 3.4 | — |
| T3.6 | 状态机：非法流转 drafted → archived → 返回错误 | 单元 | Small | 先于 3.4 | — |
| T3.7 | 状态机：非法流转 published → draft → 返回错误 | 单元 | Small | 先于 3.4 | — |
| T3.8 | ArticleRepo.Create：创建文章含标签关联 | 集成 | Medium | 先于 3.5 | repo 不存在 |
| T3.9 | ArticleRepo.FindByID：查文章并预加载标签 | 集成 | Medium | 先于 3.5 | — |
| T3.10 | ArticleRepo.List：分页 + 状态筛选 + 标签筛选 | 集成 | Medium | 先于 3.5 | — |
| T3.11 | ArticleRepo.Update：更新文章 + 标签同步 | 集成 | Medium | 先于 3.5 | — |
| T3.12 | TagRepo.FindOrCreate：tag 不存在则创建 | 集成 | Medium | 先于 3.4 | — |
| T3.13 | ArticleService.Create：作者创文章默认 draft | 单元 | Small | 先于 3.5 | — |
| T3.14 | ArticleService.Update：非本人更新 → 权限错误 | 单元 | Small | 先于 3.5 | — |
| T3.15 | ArticleService.ChangeStatus：合法流转成功 | 单元 | Small | 先于 3.5 | — |
| T3.16 | ArticleService.ChangeStatus：非法流转拒绝 | 单元 | Small | 先于 3.5 | — |
| T3.17 | GET /articles：列表只返回 published 文章 | E2E | Large | 最后 | — |
| T3.18 | GET /articles/:id：详情 +1 阅读量 | E2E | Large | 最后 | — |
| T3.19 | PATCH /articles/:id/status：作者提审 → 管理员发布 | E2E | Large | 最后 | — |

### Day 3 核心测试用例

```go
// T3.1 & T3.6: 状态机
func TestArticleStateMachine_ValidTransition(t *testing.T) {
    tests := []struct {
        from, to string
        wantErr  bool
    }{
        {"draft", "reviewing", false},
        {"draft", "published", false},
        {"reviewing", "published", false},
        {"reviewing", "draft", false},
        {"published", "archived", false},
        {"archived", "draft", false},
        // 非法
        {"draft", "archived", true},
        {"published", "draft", true},
        {"published", "reviewing", true},
        {"archived", "published", true},
    }
    for _, tt := range tests {
        t.Run(tt.from+"→"+tt.to, func(t *testing.T) {
            err := ValidateStatusTransition(tt.from, tt.to)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}

// T3.11: 更新文章时标签同步
func TestArticleRepo_Update_SyncTags(t *testing.T) {
    // Arrange: 创建文章，关联标签 ["Go", "后端"]
    // Act: 更新文章，标签改为 ["Go", "微服务"]
    // Assert: article_tags 表中只有 ("Go", "微服务")，旧的 "后端" 已移除
}
```

### Day 3 验收标准

```bash
# 状态机单元测试 — 全通过
go test ./internal/service/ -run TestArticleStateMachine -v
# PASS: TestArticleStateMachine/ValidTransition/draft→reviewing
# PASS: TestArticleStateMachine/ValidTransition/published→draft (expected error)
# ... 全部 PASS

# 集成测试
go test ./internal/repository/... -v -tags=integration
# ok  bloger/internal/repository  3.0s

# E2E: 文章全生命周期
# 作者创建草稿 → 作者提交审核 → 管理员发布 → 读者可见 → 管理员下架 → 读者不可见
```

---

## Day 4 测试计划：评论 + 点赞 + 收藏 + 敏感词

**对应开发：** Day 4 — Comment/Like/Favorite + DFA 敏感词过滤

| # | 测试内容 | 层级 | 大小 | TDD 顺序 |
|---|---|---|---|---|
| T4.1 | DFA：单个敏感词匹配 | 单元 | Small | 先于 4.4 |
| T4.2 | DFA：多个敏感词全部检出 | 单元 | Small | 先于 4.4 |
| T4.3 | DFA：无敏感词文本返回空 | 单元 | Small | 先于 4.4 |
| T4.4 | DFA：中文敏感词正确匹配 | 单元 | Small | 先于 4.4 |
| T4.5 | DFA：嵌套敏感词（如 "ab" 和 "abc"）最长匹配 | 单元 | Small | 先于 4.4 |
| T4.6 | DFA：空文本不 panic | 单元 | Small | 先于 4.4 |
| T4.7 | CommentRepo.Create：创建评论（顶级 + 回复） | 集成 | Medium | 先于 4.3 |
| T4.8 | CommentRepo.FindByArticle：查文章顶级评论 | 集成 | Medium | 先于 4.3 |
| T4.9 | CommentRepo.FindReplies：查评论子回复 | 集成 | Medium | 先于 4.3 |
| T4.10 | CommentRepo.SoftDelete：is_deleted=true | 集成 | Medium | 先于 4.3 |
| T4.11 | CommentService.BuildTree：两级树形结构正确 | 单元 | Small | 先于 4.3 |
| T4.12 | CommentService.BuildTree：嵌套 3 层，只返回 2 层 | 单元 | Small | 先于 4.3 |
| T4.13 | CommentService.Create：命中敏感词拒绝 | 单元 | Small | 先于 4.5 |
| T4.14 | CommentService.Create：无敏感词正常创建 | 单元 | Small | 先于 4.5 |
| T4.15 | LikeService.Toggle：首次点赞 created | 单元 | Small | 先于 4.6 |
| T4.16 | LikeService.Toggle：再次点赞 removed (取消) | 单元 | Small | 先于 4.6 |
| T4.17 | LikeService.Toggle：第三次点赞 created (幂等) | 单元 | Small | 先于 4.6 |
| T4.18 | FavoriteService.Toggle：收藏幂等切换（同上 3 条） | 单元 | Small | 先于 4.7 |
| T4.19 | POST /comments：包含敏感词 → 拒绝 | E2E | Large | 最后 |
| T4.20 | GET /articles/:id/comments：树形结构返回 | E2E | Large | 最后 |
| T4.21 | POST /likes：连续 3 次请求，状态切换正确 | E2E | Large | 最后 |

### Day 4 核心测试用例

```go
// T4.1-T4.6: DFA 敏感词
func TestDFA_Match_Single(t *testing.T) {
    filter := New()
    filter.AddWords("敏感词")
    result := filter.Match("这是一段包含敏感词的文本")
    assert.Equal(t, []string{"敏感词"}, result)
}

func TestDFA_Match_Multiple(t *testing.T) {
    filter := New()
    filter.AddWords("敏感词", "违禁词")
    result := filter.Match("敏感词和违禁词都出现了")
    assert.Len(t, result, 2)
}

func TestDFA_Match_NoMatch(t *testing.T) {
    filter := New()
    filter.AddWords("敏感词")
    result := filter.Match("这是正常文本")
    assert.Empty(t, result)
}

// T4.15-T4.17: 点赞幂等
func TestLikeService_Toggle_Idempotent(t *testing.T) {
    // Arrange: likeRepo 初始返回 nil（未点赞）
    // Act 1: Toggle
    // Assert: 返回 {liked: true}, Create 被调用
    //
    // Act 2: Toggle again（likeRepo 返回已存在）
    // Assert: 返回 {liked: false}, Delete 被调用
    //
    // Act 3: Toggle again（likeRepo 返回 nil）
    // Assert: 返回 {liked: true}, Create 被调用
}

// T4.11: 评论树构建
func TestCommentService_BuildTree_TwoLevels(t *testing.T) {
    // Arrange: 3 条评论
    //   c1 (顶级) → c2 (回复c1) → c3 (回复c2)
    // Act: BuildTree([c1, c2, c3])
    // Assert:
    //   返回 [{c1, replies: [{c2, replies: [{c3, replies: []}]}]}]
    //   或 [{c1, replies: [{c2, replies: []}]}, c3 在懒加载]
}
```

### Day 4 验收标准

```bash
# DFA 测试
go test ./pkg/sensitive/... -v
# PASS: TestDFA_Match_Single
# PASS: TestDFA_Match_Multiple
# PASS: TestDFA_Match_NoMatch
# PASS: TestDFA_Match_Chinese
# PASS: TestDFA_Match_Empty

# 幂等测试
go test ./internal/service/ -run TestLikeService_Toggle -v
# PASS: TestLikeService_Toggle_Idempotent

# E2E: 评论树 + 点赞切换
curl -X POST /api/v1/comments -d '{"article_id":1,"content":"好文"}'   # 成功
curl -X POST /api/v1/comments -d '{"article_id":1,"content":"敏感词"}'  # 拒绝
curl -X POST /api/v1/likes -d '{"target_type":"article","target_id":1}'  # 点赞
curl -X POST /api/v1/likes -d '{"target_type":"article","target_id":1}'  # 取消
```

---

## Day 5 测试计划：搜索 + 统计 + 限流

**对应开发：** Day 5 — 全文搜索、热门统计、令牌桶限流

| # | 测试内容 | 层级 | 大小 | TDD 顺序 |
|---|---|---|---|---|
| T5.1 | SearchRepo.FullTextSearch：关键词匹配 | 集成 | Medium | 先于 5.2 |
| T5.2 | SearchRepo：无结果返回空列表不报错 | 集成 | Medium | 先于 5.2 |
| T5.3 | SearchRepo：标题匹配权重 > 内容匹配 | 集成 | Medium | 先于 5.2 |
| T5.4 | SearchService：空关键词拒绝 | 单元 | Small | 先于 5.2 |
| T5.5 | SearchService：关键词长度 > 100 截断 | 单元 | Small | 先于 5.2 |
| T5.6 | StatsService.Trending：按 view_count 降序 | 单元 | Small | 先于 5.4 |
| T5.7 | StatsService.Trending：只统计 published | 单元 | Small | 先于 5.4 |
| T5.8 | StatsService.UserRanking：按文章数 + 评论数排序 | 单元 | Small | 先于 5.5 |
| T5.9 | RateLimit：正常频率 → 放行 | 集成 | Medium | 先于 5.6 |
| T5.10 | RateLimit：超频 → 429 | 集成 | Medium | 先于 5.6 |
| T5.11 | RateLimit：时间窗口恢复 → 再次放行 | 集成 | Medium | 先于 5.6 |
| T5.12 | RateLimit：不同接口独立计数 | 集成 | Medium | 先于 5.6 |
| T5.13 | GET /search：关键词搜索返回正确结果 | E2E | Large | 最后 |
| T5.14 | GET /stats/trending：热门文章榜单 | E2E | Large | 最后 |
| T5.15 | 限流端到端：连续请求触发 429 | E2E | Large | 最后 |

### Day 5 核心测试用例

```go
// T5.3: 标题权重 > 内容权重
func TestFullTextSearch_TitleRankedHigher(t *testing.T) {
    // Arrange: 创建两篇文章
    //   A: title="Go语言教程", content="随便写的内容"
    //   B: title="随便写",    content="这是一篇Go语言教程..."
    // Act: 搜索 "Go语言"
    // Assert: A 排在 B 前面（标题权重更高）
}

// T5.10-T5.11: 限流
func TestRateLimit_Exceeded_Returns429(t *testing.T) {
    // Arrange: login 限流 10/min
    // Act: 发 11 次请求
    // Assert: 前 10 次 200，第 11 次 429
}

func TestRateLimit_WindowReset_AllowsAgain(t *testing.T) {
    // Arrange: 打满限流配额
    // Act: 等待窗口过期 + 再发请求
    // Assert: 新的请求放行
}
```

### Day 5 验收标准

```bash
# 限流测试（Docker 内）
for i in $(seq 1 11); do
  http_code=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://<real-ip>:8080/api/v1/users/login \
    -d '{"email":"t@t.com","password":"wrong"}')
  echo "Request $i: $http_code"
done
# Request 1: 200
# ...
# Request 10: 200
# Request 11: 429

# 搜索测试
curl "http://<real-ip>:8080/api/v1/search?q=Go语言&page=1&size=10"
# 返回 published 且匹配的文章列表
```

---

## Day 6 测试计划：单元测试全覆盖

**对应开发：** Day 6 — 所有单元测试编写，覆盖率 > 70%

### Day 6 测试覆盖矩阵

| 包 | 测试文件 | 目标覆盖率 | 测试条数 | 重点关注 |
|---|---|---|---|---|
| `pkg/jwt/` | `jwt_test.go` | > 90% | ~10 | 生成/解析/过期/篡改/边界 |
| `pkg/response/` | `response_test.go` | > 90% | ~6 | 各响应函数格式 |
| `pkg/errcode/` | `errcode_test.go` | > 90% | ~3 | 唯一性/非空 |
| `pkg/sensitive/` | `dfa_test.go` | > 90% | ~8 | 单/多/嵌套/中文/空/性能 |
| `internal/service/user.go` | `user_test.go` | > 85% | ~12 | 注册/登录/校验/重复 |
| `internal/service/article.go` | `article_test.go` | > 85% | ~15 | 状态机/CRUD/权限/标签 |
| `internal/service/comment.go` | `comment_test.go` | > 80% | ~8 | 创建/树构建/敏感词 |
| `internal/service/like.go` | `like_test.go` | > 85% | ~6 | 幂等切换/多态 |
| `internal/service/favorite.go` | `favorite_test.go` | > 80% | ~5 | 幂等切换/列表分页 |
| `internal/service/search.go` | `search_test.go` | > 80% | ~6 | 校验/截断/分页 |
| `internal/service/stats.go` | `stats_test.go` | > 80% | ~5 | 排序/状态过滤 |
| `internal/middleware/auth.go` | `auth_test.go` | > 85% | ~8 | 无token/过期/正常/错误格式 |
| `internal/middleware/role.go` | `role_test.go` | > 85% | ~6 | 各角色权限边界 |
| `internal/middleware/ratelimit.go` | `ratelimit_test.go` | > 80% | ~6 | 放行/拒绝/恢复/独立计数 |
| `internal/middleware/logger.go` | `logger_test.go` | > 70% | ~3 | 日志输出格式 |
| `internal/middleware/recovery.go` | `recovery_test.go` | > 70% | ~3 | panic 恢复/500 |

### Day 6 任务清单

| # | 任务 | 预计条数 | 时间 |
|---|---|---|---|
| 6.1 | 补全 `pkg/` 下所有测试 | ~27 | 上午 |
| 6.2 | 补全 `internal/service/` 下所有测试 | ~57 | 上午 |
| 6.3 | 补全 `internal/middleware/` 下所有测试 | ~26 | 下午 |
| 6.4 | 补全 `internal/repository/` 集成测试（补齐遗漏） | ~15 | 下午 |
| 6.5 | 跑覆盖率报告，定位未覆盖代码 | - | 下午 |
| 6.6 | 补边界和异常路径用例 | ~10 | 下午 |

### Day 6 验收标准

```bash
# 全量单元测试
go test ./... -count=1 -coverprofile=coverage.out
# ?       bloger/cmd/server        [no test files]
# ok      bloger/pkg/jwt           0.3s  coverage: 92.0%
# ok      bloger/pkg/response      0.2s  coverage: 95.0%
# ok      bloger/pkg/errcode       0.2s  coverage: 90.0%
# ok      bloger/pkg/sensitive     0.3s  coverage: 88.0%
# ok      bloger/internal/service  1.2s  coverage: 83.0%
# ok      bloger/internal/middleware 2.0s coverage: 82.0%

# 总覆盖率 > 70%
go tool cover -func=coverage.out | grep total
# total: (statements)    78.5%

# 查看未覆盖代码
go tool cover -html=coverage.out -o coverage.html
```

---

## Day 7 测试计划：集成测试 + E2E 全链路 + 发布

**对应开发：** Day 7 — 集成测试脚本、GitHub 发布

| # | 测试内容 | 层级 | 大小 | 验收标准 |
|---|---|---|---|---|
| T7.1 | 全链路：注册 → 登录 → 发文 → 提审 → 发布 → 搜索 | E2E | Large | Docker 内一步不挂 |
| T7.2 | 全链路：读者浏览 → 评论 → 回复 → 点赞 → 收藏 | E2E | Large | Docker 内通过 |
| T7.3 | 状态机全路径：所有合法 + 非法流转 | E2E | Large | 合法通过，非法拦截 |
| T7.4 | 限流端到端：登录/评论/注册 各自限流 | E2E | Large | 超频返回 429 |
| T7.5 | 权限矩阵：reader/author/admin 权限边界 | E2E | Large | 越权返回 403 |
| T7.6 | 并发安全：同时注册同 email → 只有一个成功 | 集成 | Medium | UNIQUE 约束生效 |
| T7.7 | 并发安全：同时点赞 → 幂等结果正确 | 集成 | Medium | 最终状态一致 |
| T7.8 | `scripts/integration_test.sh` 一键执行 | - | - | 所有用例全部 PASS |
| T7.9 | 别人 clone 下来 `docker compose up` 直接能用 | - | - | 无需额外配置 |
| T7.10 | `make test` 跑全量测试 | - | - | 全部 GREEN |

### Day 7 E2E 测试脚本结构

```bash
#!/bin/bash
# scripts/integration_test.sh

set -e
BASE="http://<real-ip>:8080/api/v1"
PASS=0
FAIL=0

check() {
    local desc="$1" expected="$2" actual="$3"
    if echo "$actual" | grep -q "$expected"; then
        echo "✅ $desc"
        ((PASS++))
    else
        echo "❌ $desc (expected: $expected)"
        echo "   got: $actual"
        ((FAIL++))
    fi
}

echo "=== 1. 用户注册 ==="
RESP=$(curl -s -X POST "$BASE/users/register" \
    -H "Content-Type: application/json" \
    -d '{"username":"e2e_tester","email":"e2e@test.com","password":"Test1234"}')
check "注册成功" '"code":0' "$RESP"

echo "=== 2. 用户登录 ==="
TOKEN=$(curl -s -X POST "$BASE/users/login" \
    -H "Content-Type: application/json" \
    -d '{"email":"e2e@test.com","password":"Test1234"}' \
    | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
check "登录获取 token" "eyJ" "$TOKEN"

echo "=== 3. 创建文章 ==="
ARTICLE_RESP=$(curl -s -X POST "$BASE/articles" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"title":"E2E Test Article","content":"Hello World","tags":["test"]}')
check "创建文章成功" '"code":0' "$ARTICLE_RESP"
ARTICLE_ID=$(echo "$ARTICLE_RESP" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)

# ... 更多用例

echo "=== 结果 ==="
echo "通过: $PASS, 失败: $FAIL"
exit $FAIL
```

### Day 7 终极验收

```bash
# 终极验收命令
git clone <your-repo>
cd bloger
docker compose up -d
sleep 5  # 等待服务就绪
./scripts/integration_test.sh
# ✅ 注册成功
# ✅ 登录获取 token
# ✅ 创建文章成功
# ...
# 通过: 25, 失败: 0
```

---

## 测试统计总览

| 天 | 单元测试(Small) | 集成测试(Medium) | E2E(Large) | 累计 |
|---|---|---|---|---|
| Day 1 | 6 | 4 | 1 | 11 |
| Day 2 | 12 | 8 | 3 | 34 |
| Day 3 | 9 | 5 | 3 | 51 |
| Day 4 | 12 | 4 | 3 | 70 |
| Day 5 | 5 | 4 | 3 | 82 |
| Day 6 | +60 (补全) | +10 (补全) | — | ~152 |
| Day 7 | — | 2 | 5 | ~159 |

**金字塔比例：** 单元 ~125 (79%) / 集成 ~27 (17%) / E2E ~7 (4%) ✅ 符合 80/15/5

---

## 测试环境要求

```yaml
# docker-compose.test.yml — 测试专用
services:
  test-db:
    image: postgres:15-alpine
    environment:
      POSTGRES_DB: bloger_test
      POSTGRES_USER: test
      POSTGRES_PASSWORD: test
    ports:
      - "5433:5432"  # 用不同端口避免污染开发库
  
  test-redis:
    image: redis:7-alpine
    ports:
      - "6380:6379"
```

**测试隔离原则：**
- 单元测试：不依赖外部服务，mock/stub 替代
- 集成测试：使用独立 test DB + test Redis，每个测试前后清理数据
- E2E 测试：使用独立 docker compose 环境，启动前重建 DB

---

## 测试失败处理流程

```
测试失败
    │
    ├─ 是 TDD RED 阶段？ → 正常，继续写实现代码
    │
    ├─ 是回归测试失败？ → 检查最近改动，回滚定位
    │
    ├─ 是偶发性失败？ → 连续跑 3 次，仍失败则调查
    │
    └─ 未知原因？ → 检查测试数据隔离、环境依赖、时间相关逻辑
```

**红线：**
- ❌ 禁止跳过失败测试提交代码
- ❌ 禁止注释掉断言来让测试通过
- ❌ 禁止降低覆盖率阈值来凑数
