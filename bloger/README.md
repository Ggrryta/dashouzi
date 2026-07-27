# Bloger - 博客内容管理系统

技术博客平台，支持 Markdown 写作、评论互动、全文搜索、点赞收藏。

## 技术栈

| 层 | 选型 |
|---|---|
| 语言 | Go 1.23 |
| Web 框架 | Gin |
| ORM | GORM |
| 数据库 | PostgreSQL 16 |
| 缓存 | Redis 7 |
| 认证 | JWT (HS256) |
| 容器化 | Docker Compose |

## 快速启动

```bash
git clone <repo-url>
cd bloger
docker compose up -d
curl http://localhost:8080/api/v1/ping
# {"code":0,"message":"ok","data":"pong"}
```

## 运行测试

```bash
# 单元测试
go test ./... -v

# 集成测试（Docker 内）
make test-integration

# E2E 全链路
./scripts/e2e_test.sh
```

## 项目结构

```
bloger/
├── cmd/server/          # 入口
├── internal/
│   ├── config/          # 配置加载
│   ├── dto/             # 请求/响应结构体
│   ├── handler/         # HTTP 处理层
│   ├── middleware/       # 认证/限流/日志/恢复
│   ├── model/           # GORM 数据模型
│   ├── repository/      # 数据访问层
│   ├── router/          # 路由注册
│   └── service/         # 业务逻辑层
├── pkg/
│   ├── errcode/         # 统一错误码
│   ├── jwt/             # JWT 工具
│   ├── logger/          # 日志
│   ├── response/        # 统一响应
│   └── sensitive/       # DFA 敏感词过滤
├── config/              # 配置文件
├── scripts/             # 脚本
├── docker-compose.yml
├── Dockerfile
└── Makefile
```

## API 文档

### 用户模块

| 方法 | 路径 | 说明 | 认证 |
|---|---|---|---|
| POST | `/users/register` | 注册 | - |
| POST | `/users/login` | 登录 | - |
| GET | `/users/me` | 个人信息 | JWT |

### 文章模块

| 方法 | 路径 | 说明 | 权限 |
|---|---|---|---|
| POST | `/articles` | 创建 | 作者/管理员 |
| GET | `/articles` | 列表 | - |
| GET | `/articles/:id` | 详情 | - |
| PUT | `/articles/:id` | 更新 | 本人 |
| DELETE | `/articles/:id` | 删除 | 本人 |
| PATCH | `/articles/:id/status` | 状态变更 | 本人 |

### 评论模块

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/comments` | 发表 |
| DELETE | `/comments/:id` | 删除 |
| GET | `/articles/:id/comments` | 评论树 |

### 点赞 & 收藏

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/likes` | 点赞/取消 |
| GET | `/likes/check` | 查状态 |
| POST | `/favorites` | 收藏/取消 |
| GET | `/favorites` | 收藏列表 |

### 搜索 & 统计

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/search?q=` | 全文搜索 |
| GET | `/stats/trending` | 热门文章 |
| GET | `/stats/users` | 用户排行 |

## 状态机

```
draft → reviewing → published → archived
  ↑        ↓
  └── 驳回 ──┘
```

## 限流策略

| 接口 | 限制 |
|---|---|
| 登录/注册 | 30次/分钟 |
| 评论 | 20次/分钟 |
