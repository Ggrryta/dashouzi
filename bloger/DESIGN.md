# 博客内容管理系统 - 设计开发文档

## 1. 项目概述

一个支持 Markdown 写作、评论互动、全文搜索的技术博客平台。模拟公司内部知识沉淀场景。

## 2. 技术栈

| 层 | 选型 | 说明 |
|---|---|---|
| 语言 | Go 1.21+ | 高性能，大厂主流 |
| Web 框架 | Gin | 轻量高性能 |
| ORM | GORM | Go 最流行 ORM |
| 数据库 | PostgreSQL 15 | 支持全文索引，功能强大 |
| 缓存 | Redis 7 | 热点数据缓存、限流计数 |
| 认证 | JWT (golang-jwt) | 无状态鉴权 |
| 容器化 | Docker Compose | 一键启动 app + PG + Redis |

## 3. 数据库设计

### 3.1 ER 关系

```
users ──1:N──> articles ──1:N──> comments
  │               │                 │
  │               │                 └── N:1 ──> comments (parent, 嵌套回复)
  │               │
  │               └── N:N ──> tags (via article_tags)
  │
  ├── 1:N ──> likes (多态: like_type + target_id)
  │
  └── 1:N ──> favorites (收藏)
```

### 3.2 表结构

```sql
-- 用户表
CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    username      VARCHAR(64)  NOT NULL UNIQUE,
    email         VARCHAR(128) NOT NULL UNIQUE,
    password_hash VARCHAR(256) NOT NULL,
    role          VARCHAR(16)  NOT NULL DEFAULT 'reader',  -- admin / author / reader
    avatar_url    VARCHAR(512),
    bio           TEXT,
    created_at    TIMESTAMP    NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMP    NOT NULL DEFAULT NOW()
);

-- 文章表
CREATE TABLE articles (
    id           BIGSERIAL PRIMARY KEY,
    author_id    BIGINT       NOT NULL REFERENCES users(id),
    title        VARCHAR(256) NOT NULL,
    slug         VARCHAR(256) NOT NULL UNIQUE,  -- URL 友好的唯一标识
    content      TEXT         NOT NULL,
    summary      VARCHAR(512),
    cover_url    VARCHAR(512),
    status       VARCHAR(16)  NOT NULL DEFAULT 'draft',  -- draft / reviewing / published / archived
    view_count   BIGINT       NOT NULL DEFAULT 0,
    is_top       BOOLEAN      NOT NULL DEFAULT FALSE,
    published_at TIMESTAMP,
    created_at   TIMESTAMP    NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMP    NOT NULL DEFAULT NOW()
);

-- 文章状态机流转规则:
-- draft --> reviewing (提交审核)
-- reviewing --> published (审核通过) | draft (审核驳回)
-- published --> archived (下架)
-- 除 draft->reviewing 驳回外，所有流转不可逆

-- 标签表
CREATE TABLE tags (
    id         BIGSERIAL PRIMARY KEY,
    name       VARCHAR(64) NOT NULL UNIQUE,
    created_at TIMESTAMP   NOT NULL DEFAULT NOW()
);

-- 文章-标签关联表
CREATE TABLE article_tags (
    article_id BIGINT NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    tag_id     BIGINT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (article_id, tag_id)
);

-- 评论表（支持嵌套回复）
CREATE TABLE comments (
    id          BIGSERIAL PRIMARY KEY,
    article_id  BIGINT    NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    user_id     BIGINT    NOT NULL REFERENCES users(id),
    parent_id   BIGINT    REFERENCES comments(id) ON DELETE CASCADE,  -- NULL=顶级评论
    content     TEXT      NOT NULL,
    is_deleted  BOOLEAN   NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMP NOT NULL DEFAULT NOW()
);

-- 点赞表（多态设计，可扩展到文章/评论点赞）
CREATE TABLE likes (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT      NOT NULL REFERENCES users(id),
    target_type VARCHAR(16) NOT NULL,  -- 'article' / 'comment'
    target_id  BIGINT      NOT NULL,
    created_at TIMESTAMP   NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, target_type, target_id)
);

-- 收藏表
CREATE TABLE favorites (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT    NOT NULL REFERENCES users(id),
    article_id BIGINT    NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, article_id)
);

-- 全文搜索索引
ALTER TABLE articles ADD COLUMN search_vector tsvector
    GENERATED ALWAYS AS (
        setweight(to_tsvector('simple', coalesce(title, '')), 'A') ||
        setweight(to_tsvector('simple', coalesce(content, '')), 'B')
    ) STORED;

CREATE INDEX articles_search_idx ON articles USING GIN (search_vector);
```

## 4. API 设计

所有接口前缀 `/api/v1`，返回格式统一：

```json
{
    "code": 0,
    "message": "ok",
    "data": {}
}
```

### 4.1 用户模块

| 方法 | 路径 | 说明 | 认证 |
|---|---|---|---|
| POST | `/users/register` | 注册 | 否 |
| POST | `/users/login` | 登录，返回 JWT | 否 |
| GET | `/users/me` | 获取当前用户信息 | 是 |
| PUT | `/users/me` | 更新个人信息 | 是 |
| GET | `/users/:id` | 查看用户公开信息 | 否 |

### 4.2 文章模块

| 方法 | 路径 | 说明 | 认证 | 权限 |
|---|---|---|---|---|
| POST | `/articles` | 创建文章 | 是 | author/admin |
| PUT | `/articles/:id` | 更新文章 | 是 | 本人/admin |
| DELETE | `/articles/:id` | 删除文章 | 是 | 本人/admin |
| GET | `/articles/:id` | 查看文章详情 | 否 | - |
| GET | `/articles` | 文章列表（分页+搜索+标签筛选） | 否 | - |
| PATCH | `/articles/:id/status` | 变更文章状态 | 是 | 本人/admin |
| PATCH | `/articles/:id/top` | 置顶/取消置顶 | 是 | admin |
| GET | `/articles/:id/comments` | 文章评论列表（树形结构） | 否 | - |

### 4.3 评论模块

| 方法 | 路径 | 说明 | 认证 |
|---|---|---|---|
| POST | `/comments` | 发表评论（支持 parent_id） | 是 |
| DELETE | `/comments/:id` | 删除评论（软删除） | 是（本人/admin） |
| GET | `/comments/:id/replies` | 获取评论的回复 | 否 |

### 4.4 点赞 & 收藏

| 方法 | 路径 | 说明 | 认证 |
|---|---|---|---|
| POST | `/likes` | 点赞/取消点赞（幂等切换） | 是 |
| GET | `/articles/:id/like-status` | 当前用户是否已点赞 | 是 |
| POST | `/favorites` | 收藏/取消收藏 | 是 |
| GET | `/favorites` | 我的收藏列表 | 是 |

### 4.5 搜索 & 统计

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/search` | 全文搜索（q=关键词, page, size） |
| GET | `/tags` | 标签列表（含文章数） |
| GET | `/stats/trending` | 热门文章（周榜/月榜） |
| GET | `/stats/users` | 用户活跃度排行 |

## 5. 项目目录结构

```
bloger/
├── DESIGN.md                # 本文档
├── docker-compose.yml       # 一键启动
├── Dockerfile               # Go 应用镜像
├── go.mod
├── go.sum
├── Makefile                 # 常用命令快捷方式
├── cmd/
│   └── server/
│       └── main.go          # 入口
├── internal/
│   ├── config/
│   │   └── config.go        # 配置加载（环境变量 + yaml）
│   ├── model/
│   │   ├── user.go
│   │   ├── article.go
│   │   ├── comment.go
│   │   ├── tag.go
│   │   ├── like.go
│   │   └── favorite.go
│   ├── handler/
│   │   ├── user.go          # 用户接口处理
│   │   ├── article.go       # 文章接口处理
│   │   ├── comment.go       # 评论接口处理
│   │   ├── like.go          # 点赞接口处理
│   │   ├── favorite.go      # 收藏接口处理
│   │   ├── search.go        # 搜索接口处理
│   │   └── stats.go         # 统计接口处理
│   ├── service/
│   │   ├── user.go
│   │   ├── article.go
│   │   ├── comment.go
│   │   ├── like.go
│   │   ├── search.go
│   │   └── stats.go
│   ├── repository/           # 数据访问层（GORM）
│   │   ├── user.go
│   │   ├── article.go
│   │   ├── comment.go
│   │   ├── tag.go
│   │   ├── like.go
│   │   └── favorite.go
│   ├── middleware/
│   │   ├── auth.go          # JWT 认证中间件
│   │   ├── role.go          # 角色鉴权中间件
│   │   ├── ratelimit.go     # 令牌桶限流中间件
│   │   ├── logger.go        # 请求日志中间件
│   │   └── recovery.go      # panic 恢复中间件
│   ├── router/
│   │   └── router.go        # 路由注册
│   └── dto/                  # 请求/响应结构体
│       ├── request.go
│       └── response.go
├── pkg/
│   ├── jwt/
│   │   └── jwt.go           # JWT 生成/校验
│   ├── errcode/
│   │   └── errcode.go       # 统一错误码定义
│   ├── response/
│   │   └── response.go      # 统一响应封装
│   └── sensitive/
│       └── dfa.go           # DFA 敏感词过滤
├── config/
│   └── config.yaml          # 默认配置
└── scripts/
    ├── init.sql              # 数据库初始化 SQL
    └── test.sh               # 集成测试脚本
```

## 6. 关键技术点

### 6.1 文章状态机

```
                    ┌──────────┐
          提交审核   │  draft   │  审核驳回
       ┌──────────> │  (草稿)   │ <──────────┐
       │            └──────────┘            │
       │                  │                 │
       │                  │ 审核通过         │
       │                  ▼                 │
       │            ┌──────────┐           │
       │            │reviewing │           │
       │            │ (审核中)  │           │
       │            └──────────┘           │
       │                  │                 │
       │                  │ 审核通过         │
       │                  ▼                 │
       │            ┌──────────┐           │
       │            │published │           │
       │            │ (已发布)  │           │
       │            └──────────┘           │
       │                  │                 │
       │                  │ 下架            │
       │                  ▼                 │
       │            ┌──────────┐           │
       └─────────── │archived  │ ──────────┘
        审核驳回     │ (已下架)  │
                    └──────────┘
```

合法流转：
- `draft` → `reviewing`（作者提交审核）
- `draft` → `published`（管理员直接发布）
- `reviewing` → `published`（审核通过）
- `reviewing` → `draft`（审核驳回，退回修改）
- `published` → `archived`（下架）
- `archived` → `draft`（重新编辑为草稿）

对外规则：只有 `published` 状态的文章对读者可见。

### 6.2 JWT 认证 + RBAC

- JWT 包含：`user_id`, `username`, `role`
- 三个角色：`admin`（管理员）、`author`（作者）、`reader`（读者）
- 中间件链：`auth (解析JWT)` → `role (检查角色)` → `handler`
- 读者：只能浏览、评论、点赞、收藏
- 作者：额外可创建/管理自己的文章
- 管理员：所有权限

### 6.3 DFA 敏感词过滤

对评论内容进行敏感词过滤，使用 DFA（Deterministic Finite Automaton）算法：
- 维护敏感词库（Trie 树结构）
- O(n) 时间复杂度，n 为文本长度
- 支持中文敏感词匹配
- 命中后返回敏感词列表，由调用方决定是拦截还是替换

### 6.4 令牌桶限流

对登录、注册、评论等接口进行限流：
- 使用 Redis 实现滑动窗口 + 令牌桶
- 每个 IP + 接口路径作为 key
- 登录接口：10次/分钟/IP
- 评论接口：20次/分钟/用户
- 超过限制返回 429 Too Many Requests

### 6.5 全文搜索

- 使用 PostgreSQL 的 `tsvector` + `tsquery` 实现
- 标题权重 A > 内容权重 B
- 支持中文分词（需要 pg_jieba 或 zhparser 扩展，或用 simple 配置）
- 搜索结果按相关度排序

### 6.6 评论树形结构

- 使用 `parent_id` 自关联，支持无限层级嵌套
- 列表接口返回两级：顶级评论 + 子回复列表
- 更深层级的回复通过"加载更多"接口懒加载
- 软删除：`is_deleted = true`，已删除的评论显示"该评论已删除"

## 7. 开发顺序（按天）

### Day 1：项目骨架 + 用户模块
- 初始化 Go module、Gin 项目骨架
- 写 Dockerfile + docker-compose.yml（PG + Redis）
- 实现用户注册/登录（JWT）
- 实现 auth 中间件

### Day 2：文章 CRUD + 状态机
- 文章创建、编辑、删除
- 状态机流转逻辑
- 文章列表分页查询
- 标签管理

### Day 3：评论 + 点赞 + 收藏
- 评论 CRUD（含嵌套回复、树形返回）
- 点赞/收藏的幂等切换
- DFA 敏感词过滤集成

### Day 4：搜索 + 统计 + 限流
- PostgreSQL 全文搜索
- 热门文章、用户活跃度统计
- 令牌桶限流中间件
- 所有中间件串联

### Day 5：测试 + 联调
- 单元测试（覆盖率 > 70%）
- 集成测试（Docker 环境）
- 接口文档整理
