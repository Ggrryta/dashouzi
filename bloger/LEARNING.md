# 博客系统 - 能力沉淀文档

> 本文档梳理你通过实现这个博客系统掌握的核心后端能力，按面试考察维度组织。
> 每个知识点都包含：**你做了什么 → 为什么这样做 → 面试怎么讲**。

---

## 1. 项目架构能力

### 三层分离：Handler → Service → Repository

```
请求 → Handler(参数校验+响应) → Service(业务逻辑) → Repository(数据库操作)
         ↑ 不能反向调用        ↑ 不依赖 HTTP 框架    ↑ 可被 mock 替换
```

**为什么这样做：**
- 每一层职责单一，改动不波及上下层
- Service 依赖 Repository 接口而非具体实现，单元测试时直接注入 mock
- 面试官看到你能写出可测试的架构，证明你不是"面条代码"选手

**面试话术：**
> "我采用 handler-service-repository 三层架构，service 层通过接口依赖 repository，
> 这使得我在单元测试中可以用 mock 完全替换数据库，测试覆盖率做到了 83% 以上。"

### 接口式编程

你定义了 `UserRepository`、`ArticleRepository` 等多个接口：

```go
type UserRepository interface {
    Create(ctx context.Context, user *model.User) error
    FindByEmail(ctx context.Context, email string) (*model.User, error)
    ...
}
```

**面试话术：**
> "通过 interface 定义 repository 契约，service 不直接依赖 gorm.DB，方便测试时注入 mock，
> 也方便未来切换 ORM 或数据库。"

---

## 2. 认证与授权

### JWT 无状态认证

```go
// 签发
claims := Claims{UserID: userID, Username: username, Role: role}
token := jwt.NewWithClaims(HS256, claims).SignedString(secret)

// 校验
token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, keyFunc)
```

**核心理解：**
- Header.Payload.Signature 三段式结构
- HS256：对称加密，签发和校验用同一密钥
- Token 存客户端，服务端无状态，天然支持水平扩展
- 失效依赖过期时间，不像 Session 可以服务端主动踢人

**面试话术：**
> "JWT 的优势是无状态，服务端不需要存 session，天然支持水平扩展。
> 但缺点是无法主动失效，所以我们设置了较短的过期时间（24小时），搭配限流防止爆破。"

### RBAC 角色权限模型

你实现了三级角色层级：

```go
var roleHierarchy = map[string]int{
    "reader": 1,  // 只能浏览、评论、点赞
    "author": 2,  // 额外：管理自己的文章
    "admin":  3,  // 所有权限
}
```

**面试话术：**
> "通过数字层级实现 RBAC，hasAccess 比较当前角色等级是否 >= 要求等级。
> 这样 admin 自动拥有 author 和 reader 的权限，不需要在每个接口重复配置。"

### 中间件链

```go
// 请求经过的中间件顺序
Logger → Recovery → RateLimit → Auth → Role → Handler
```

**面试话术：**
> "中间件按洋葱模型串联，越底层的越外层。
> Auth 解析 token 后将 user_id/role 写入 context，后续中间件和 handler 直接从 context 读取，
> 实现了关注点分离。"

---

## 3. 状态机设计

### 文章状态流转

```
draft → reviewing → published → archived
  ↑        ↓
  └── 驳回 ──┘
```

**实现方式：**

```go
var validTransitions = map[string]map[string]bool{
    "draft":     {"reviewing": true, "published": true},
    "reviewing": {"published": true, "draft": true},       // 驳回回退
    "published": {"archived": true},
    "archived":  {"draft": true},                           // 重新编辑
}
```

**面试话术：**
> "用配置表（map[string]map[string]bool）而非 if-else 实现状态机，新增状态只需加一行配置。
> 状态流转有严格的方向约束，比如 published 不能回到 draft，保证业务合规。"

---

## 4. 数据一致性与幂等

### 点赞/收藏的幂等设计

同一接口 POST 两次：第一次创建，第二次删除。

```go
func Toggle(ctx, userID, targetType, targetID) (bool, error) {
    exists, _ := repo.Exists(ctx, userID, targetType, targetID)
    if exists {
        repo.Delete(...)
        return false, nil  // 已取消
    }
    repo.Create(...)
    return true, nil  // 已点赞
}
```

**面试话术：**
> "采用先查后写（check-then-act）实现幂等切换。数据库层用 UNIQUE 约束兜底，
> 即使并发请求，唯一索引也能阻止重复插入。"

### 软删除

评论删除不是物理删除，而是 `is_deleted = true`：

**面试话术：**
> "评论使用软删除，保留数据用于审计和数据分析。查询时过滤 is_deleted = false，
> 但原有回复关系不丢失，已删除评论显示为'该评论已删除'。"

---

## 5. DFA 敏感词过滤

### 算法原理

```
Trie 树结构，O(n) 时间复杂度（n 为文本长度），与敏感词库大小无关
```

```
            root
           /    \
         '违'   '敏'
         /        \
       '禁'      '感'
       /           \
     '词'(end)    '词'(end)
```

**面试话术：**
> "DFA 算法的核心是 Trie 树 + 状态转移。每个字符是一步状态转移，走到 isEnd 节点就命中。
> 时间复杂度 O(n)，比正则 O(n*m) 快得多。支持最长匹配，'abc' 不会重复命中 'ab'。"

**为什么不用正则：** 正则回溯可能导致 ReDoS 攻击，且当敏感词库上万时性能急剧下降。

---

## 6. 限流系统

### 固定窗口算法

```go
type rateEntry struct {
    count  int       // 当前窗口内计数
    window time.Time // 窗口起始时间
}
```

**面试话术：**
> "实现了固定窗口计数器限流。每个 IP+Path 组合独立计数，超限返回 429。
> 登录接口 30次/分钟，评论接口 20次/分钟。窗口过期后计数器自动重置。"

**限流的三种算法比较（面试常考）：**

| 算法 | 原理 | 优点 | 缺点 |
|---|---|---|---|
| 固定窗口 | 每个时间窗口内计数 | 实现简单 | 边界突刺 |
| 滑动窗口 | 记录每次请求时间戳 | 平滑 | 内存开销大 |
| 令牌桶 | 固定速率放令牌 | 允许突发 | 实现复杂 |

---

## 7. 测试体系

### 测试金字塔实践

```
         ╱╲
        ╱ E2E ╲      24 个全链路用例，scripts/e2e_test.sh
       ╱────────╲
      ╱  集成测试 ╲    10 个 DB 集成测试，Docker 内运行
     ╱────────────╲
    ╱   单元测试    ╲   50+ 个 mock 测试，毫秒级
   ╱────────────────╲
```

**面试话术：**
> "遵循测试金字塔：80% 单元测试（mock service/repo）、15% 集成测试（真实 PG）、
> 5% E2E 测试（全链路 HTTP）。集成和 E2E 都在 Docker 内运行，保证环境一致性。"

### Mock 策略

```go
// 定义接口
type UserRepository interface { ... }

// 测试用 mock
type mockUserRepo struct { users map[string]*model.User }

// 注入
svc := NewUserService(mockRepo)  // 测试
svc := NewUserService(realRepo)  // 生产
```

**面试话术：**
> "通过 interface 解耦，测试时注入内存 mock，不依赖数据库。
> Mock 模拟真实行为（邮箱冲突、用户不存在等边界），而不是简单的返回 nil。"

---

## 8. Docker 工程化

### 多阶段构建

```dockerfile
FROM golang:1.23-alpine AS builder    # 编译阶段（~300MB）
COPY . .
RUN go build -o /server

FROM alpine:3.18                       # 运行阶段（~15MB）
COPY --from=builder /server .
```

**面试话术：**
> "多阶段构建使最终镜像只有 15MB，不包含编译工具链。
> 使用 vendor 离线构建，Docker 内不依赖外部网络下载依赖。"

### Docker Compose 编排

```yaml
postgres:
  healthcheck:
    test: ["CMD-SHELL", "pg_isready -U bloger"]
app:
  depends_on:
    postgres:
      condition: service_healthy  # 等 PG 就绪再启动
```

**面试话术：**
> "depends_on 加 healthcheck 条件，确保 PG 真正就绪后 app 才启动，
> 避免启动顺序竞争导致的连接失败。"

---

## 9. 面试高频问答速查表

| 面试问题 | 你的答案要点 |
|---|---|
| **JWT vs Session？** | 无状态 → 水平扩展容易；无法主动失效 → 短过期+限流 |
| **如何防止超卖？** | （博客没做，但秒杀系统 Day 2 有）Redis Lua 原子扣减 |
| **状态机怎么实现的？** | map[string]map[string]bool 配置表，O(1) 校验 |
| **DFA 时间复杂度？** | O(n)，n=文本长度，与词库大小无关 |
| **点赞怎么保证幂等？** | check-then-act + DB UNIQUE 约束兜底 |
| **RBAC 怎么设计？** | 数字层级：admin(3) > author(2) > reader(1) |
| **评论树怎么查？** | parent_id 自关联 + Preload 两级，深层次懒加载 |
| **限流用什么算法？** | 固定窗口计数器，IP+Path 组合 key |
| **怎么保证代码可测试？** | interface + 依赖注入，mock 替换真实 DB |
| **Docker 为什么多阶段？** | 编译阶段 ~300MB → 运行阶段 ~15MB |

---

## 10. 能力模型对照

| 训练目标 | 实现方式 |
|---|---|
| RESTful API 设计 | 资源化 URL，统一 {code,message,data} 响应 |
| 数据库范式设计 | 7 表 3NF，多对多关联表 |
| JWT + RBAC | HS256 签名 + 角色层级中间件 |
| 状态机模式 | map 配置表驱动的状态流转 |
| DFA 算法 | Trie 树的工程实现 |
| 令牌桶限流 | 固定窗口计数器 |
| 三层架构 | Handler→Service→Repository |
| 接口+依赖注入 | interface 定义契约，mock 测试 |
| Docker 工程化 | 多阶段构建 + healthcheck + 离线 vendor |
| 测试金字塔 | 单元(80%)+集成(15%)+E2E(5%) |

---

## 11. 简历建议

**项目描述：**

> 基于 Go + Gin + PostgreSQL 的博客内容管理系统，支持 Markdown 写作、评论互动、
> DFA 敏感词过滤、角色权限控制、全文搜索、令牌桶限流。
> 采用 handler-service-repository 三层架构，接口依赖注入，测试覆盖率 83%+。
> Docker Compose 一键部署，包含完整的集成测试和 E2E 自动化测试。

**技术亮点关键词：**

`Go` `Gin` `GORM` `PostgreSQL` `JWT` `RBAC` `状态机` `DFA` `幂等设计`
`全文搜索` `限流` `Docker` `三层架构` `依赖注入` `单元测试` `集成测试` `E2E`
