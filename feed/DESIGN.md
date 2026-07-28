# Feed 流系统设计文档

## 1. 概述

Feed 流系统是社交平台的核心功能，负责为每个用户生成个性化的动态时间线。本系统采用 **推拉结合 (Push-Pull Hybrid)** 架构，兼顾实时性与系统开销。

## 2. 核心架构

### 2.1 推拉结合模型

```
                        ┌──────────────────────┐
                        │     Feed Service      │
                        └──────────┬───────────┘
                                   │
              ┌────────────────────┼────────────────────┐
              │                    │                    │
     ┌────────▼────────┐  ┌───────▼───────┐  ┌────────▼────────┐
     │  写扩散 (Push)   │  │  读扩散 (Pull) │  │  冷热分离       │
     │  普通用户发帖     │  │  大V 发帖      │  │  归档策略       │
     └────────┬────────┘  └───────┬───────┘  └────────┬────────┘
              │                    │                    │
     ┌────────▼────────┐  ┌───────▼───────┐  ┌────────▼────────┐
     │  Redis List      │  │  发件箱模式    │  │  MySQL/归档     │
     │  粉丝收件箱       │  │  粉丝拉取      │  │  冷数据存储     │
     └─────────────────┘  └───────────────┘  └─────────────────┘
```

### 2.2 分发策略

| 用户类型 | 粉丝阈值 | 策略 | 说明 |
|---------|---------|------|------|
| 普通用户 | < 10万 | **写扩散 (Push)** | 发帖时写入粉丝收件箱 (Redis List) |
| 大V | >= 10万 | **读扩散 (Pull)** | 不发收件箱，粉丝主动拉取发件箱 |

### 2.3 数据存储设计

**Redis 存储结构：**

| Key | 类型 | 说明 |
|-----|------|------|
| `timeline:{user_id}` | Sorted Set | 用户时间线（收件箱），score=发帖时间戳 |
| `outbox:{user_id}` | Sorted Set | 用户发件箱（大V模式），score=发帖时间戳 |
| `feed:hot` | Sorted Set | 热门 Feed 全局排行 |
| `user:following:{user_id}` | Set | 用户关注列表 |

**MySQL 存储：**

```sql
-- 帖子表
CREATE TABLE posts (
    id BIGINT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    content TEXT NOT NULL,
    is_big_v TINYINT DEFAULT 0,  -- 发帖时的状态快照
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_user_created (user_id, created_at)
);

-- 关注关系表
CREATE TABLE follows (
    follower_id BIGINT NOT NULL,
    followee_id BIGINT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (follower_id, followee_id),
    INDEX idx_followee (followee_id)
);

-- 大V 标记表
CREATE TABLE big_v_users (
    user_id BIGINT PRIMARY KEY,
    follower_count INT DEFAULT 0,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

## 3. 核心流程

### 3.1 发帖流程

```
POST /api/v1/posts
     │
     ▼
┌─────────────┐     ┌──────────────┐
│ 1. 写入 MySQL │────▶│ 2. 写入发件箱  │
└─────────────┘     │   outbox:{uid}│
                    └──────┬───────┘
                           │
              ┌────────────┼────────────┐
              │ 普通用户    │            │ 大V
              ▼            │            ▼
     ┌──────────────┐      │    ┌──────────────┐
     │ 3a. 查粉丝列表 │      │    │ 3b. 不做扩散   │
     │ 批量写收件箱   │      │    │ 仅更新发件箱   │
     │ timeline:{fid}│      │    └──────────────┘
     └──────┬───────┘      │
            │              │
            ▼              │
     ┌──────────────┐      │
     │ 4. 异步扩散    │      │
     │ 限制队列长度   │      │
     │ (最多1000条)   │      │
     └──────────────┘      │
```

### 3.2 拉取 Timeline 流程

```
GET /api/v1/timeline?cursor={timestamp}&limit=20
     │
     ▼
┌──────────────────────┐
│ 1. 读 Redis 收件箱    │
│    timeline:{uid}     │
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│ 2. 收件箱数据够?       │
│    len >= limit?      │
└──────────┬───────────┘
           │
     ┌─────┼─────┐
     │ 够  │     │ 不够
     ▼     │     ▼
  返回     │  ┌──────────────────────┐
           │  │ 3. 拉取关注的 大V 发件箱│
           │  │    outbox:{big_v_id}  │
           │  └──────────┬───────────┘
           │             │
           │             ▼
           │  ┌──────────────────────┐
           │  │ 4. Merge + 按时间排序  │
           │  │    返回 Top N          │
           │  └──────────────────────┘
```

### 3.3 关注/取关流程

```
POST /api/v1/follow
     │
     ▼
┌──────────────────────┐
│ 1. 写入 MySQL        │
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│ 2. 检查被关注者是否大V │
└──────────┬───────────┘
           │
     ┌─────┼─────┐
     │ 大V │     │ 普通
     ▼     │     ▼
  无需同步 │  ┌──────────────────────┐
           │  │ 3. 拉近30天发件箱     │
           │  │ 写入新粉收件箱        │
           │  └──────────────────────┘
```

## 4. 技术选型

| 组件 | 技术 | 说明 |
|------|------|------|
| 语言 | Go 1.21+ | 高性能并发 |
| HTTP 框架 | Gin | 路由 + 中间件 |
| 数据库 | MySQL 8.0 | 持久化存储 |
| 缓存 | Redis 7 | Timeline / 发件箱 / 关注关系 |
| 消息队列 | Redis Streams | 异步写扩散 |
| 容器化 | Docker Compose | 开发环境 |

## 5. API 设计

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/posts` | 发布帖子 |
| GET | `/api/v1/timeline` | 拉取个人时间线 |
| GET | `/api/v1/posts/:id` | 帖子详情 |
| POST | `/api/v1/follow` | 关注用户 |
| POST | `/api/v1/unfollow` | 取关用户 |
| GET | `/api/v1/following` | 关注列表 |
| GET | `/api/v1/followers` | 粉丝列表 |
| GET | `/api/v1/feed/hot` | 热门 Feed |

### 请求/响应示例

**发布帖子：**
```json
POST /api/v1/posts
{
    "user_id": 123,
    "content": "Hello, World!"
}
Response:
{
    "code": 0,
    "data": {
        "post_id": 456,
        "created_at": "2026-07-28T10:30:00Z"
    }
}
```

**拉取时间线：**
```json
GET /api/v1/timeline?user_id=123&limit=20&cursor=1722155400
Response:
{
    "code": 0,
    "data": {
        "items": [
            {
                "post_id": 456,
                "user_id": 789,
                "content": "...",
                "created_at": "2026-07-28T10:30:00Z"
            }
        ],
        "next_cursor": "1722155300",
        "has_more": true
    }
}
```

## 6. 关键设计决策

1. **推拉阈值 10万粉丝**：普通用户写扩散确保实时性；大V读扩散避免写扩散风暴
2. **收件箱上限 1000 条**：Redis List 只保留最近 N 条，更早数据走 MySQL
3. **异步扩散**：发帖后异步写入粉丝收件箱，不阻塞主流程
4. **Cursor 分页**：使用时间戳游标，避免深分页问题
5. **发帖时快照大V状态**：避免历史数据因用户状态变更而不一致
