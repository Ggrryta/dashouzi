# Feed 流系统 API 文档

## 基础信息

- Base URL: `http://localhost:8080`
- Content-Type: `application/json`

## 响应格式

```json
{
  "code": 0,       // 0=成功, -1=失败
  "msg": "...",    // 错误信息
  "data": { ... }  // 响应数据
}
```

---

## 1. 健康检查

```
GET /health
```

**响应：**
```json
{"status": "ok"}
```

---

## 2. 帖子

### 2.1 发布帖子

```
POST /api/v1/posts
```

**请求体：**
```json
{
  "user_id": 1,
  "content": "Hello, Feed!"
}
```

**成功响应 (200)：**
```json
{
  "code": 0,
  "data": {
    "post_id": 123456,
    "created_at": "2026-07-28T10:30:00Z"
  }
}
```

**错误响应：**
| 场景 | HTTP | msg |
|------|------|-----|
| user_id 缺失 | 400 | `Key: 'createPostReq.UserID' Error:Field validation for 'UserID' failed on the 'required' tag` |
| content 为空 | 400 | `content is required` |

### 2.2 获取帖子详情

```
GET /api/v1/posts/:id
```

**路径参数：**
- `id` — 帖子ID

**成功响应 (200)：**
```json
{
  "code": 0,
  "data": {
    "id": 123456,
    "user_id": 1,
    "content": "Hello, Feed!",
    "is_big_v": false,
    "created_at": "2026-07-28T10:30:00Z"
  }
}
```

**错误响应：**
| 场景 | HTTP | msg |
|------|------|-----|
| id 格式错误 | 400 | `invalid id` |
| 帖子不存在 | 404 | `post not found` |

---

## 3. 时间线

### 3.1 拉取个人时间线

```
GET /api/v1/timeline?user_id=1&limit=20&cursor=1234567890
```

**查询参数：**

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:---:|--------|------|
| `user_id` | int64 | 是 | — | 用户ID |
| `limit` | int | 否 | 20 | 每页条数（上限 100） |
| `cursor` | string | 否 | — | 分页游标（上次返回的 next_cursor），首次请求不传 |

**成功响应 (200)：**
```json
{
  "code": 0,
  "data": {
    "items": [
      {
        "post_id": 456,
        "user_id": 789,
        "content": "帖子内容",
        "timestamp": 1722155400,
        "created_at": "2026-07-28T10:30:00Z"
      }
    ],
    "next_cursor": "1722155300",
    "has_more": true
  }
}
```

**字段说明：**
- `items` — 按时间倒序排列的帖子列表
- `next_cursor` — 下一页游标，传给下个请求的 `cursor` 参数
- `has_more` — `true` 表示还有更多数据

> **拉取逻辑**：先从收件箱（Push）读取，不足时自动拉取关注的大V发件箱（Pull），合并去重后返回。

---

## 4. 社交关系

### 4.1 关注用户

```
POST /api/v1/follow
```

**请求体：**
```json
{
  "follower_id": 1,
  "followee_id": 2
}
```

**成功响应 (200)：**
```json
{"code": 0, "data": "ok"}
```

**错误：**
| 场景 | msg |
|------|-----|
| 关注自己 | `cannot follow self` |
| 缺少参数 | 必填验证错误 |

### 4.2 取消关注

```
POST /api/v1/unfollow
```

**请求体：**
```json
{
  "follower_id": 1,
  "followee_id": 2
}
```

**成功响应 (200)：**
```json
{"code": 0, "data": "ok"}
```

### 4.3 获取关注列表

```
GET /api/v1/following?user_id=1
```

**响应：**
```json
{
  "code": 0,
  "data": [2, 3, 4]
}
```

### 4.4 获取粉丝列表

```
GET /api/v1/followers?user_id=2
```

**响应：**
```json
{
  "code": 0,
  "data": [1, 5, 6]
}
```

---

## 架构说明

### 推拉结合策略

| 用户类型 | 粉丝阈值 | 发帖行为 | 拉时间线行为 |
|---------|---------|---------|------------|
| 普通用户 | < 10万 | 异步写扩散到粉丝收件箱 (Redis Sorted Set) | 直接从收件箱读取 |
| 大V | >= 10万 | 仅写入自己的发件箱，不做扩散 | Pull 大V发件箱 + 合并排序 |

### 数据流

```
发帖 → OutBox + [普通用户→异步Diffusion→粉丝Timeline / 大V→仅OutBox]
拉取 → Timeline(Push) + OutBox(Pull for BigVs) → Merge → 返回
```
