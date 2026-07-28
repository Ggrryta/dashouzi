# IM - 多节点实时聊天系统

基于 Go + WebSocket + Redis Pub/Sub + Kafka 的多节点 IM 系统。

## 技术栈

| 层 | 选型 | 说明 |
|---|---|---|
| WebSocket | gorilla/websocket | 长连接管理 |
| 网关 | Gin + JWT | 认证 + 一致性哈希分配节点 |
| 消息路由 | Redis 7 Pub/Sub | 跨节点实时转发 |
| 离线消息 | Kafka | 异步投递 |
| 数据库 | MySQL 8.0 | 消息历史 |
| 负载均衡 | 一致性哈希 | 用户黏性 |

## 架构

```
Gateway(认证+分配) → 3×IMNode(WebSocket) → Redis Pub/Sub(跨节点) → Kafka(离线)
```

## 快速启动

```bash
docker compose up -d
curl http://localhost:8080/api/v1/ping
```

## API

| 协议 | 路径 | 说明 |
|---|---|---|
| HTTP POST | `/api/v1/register` | 注册 |
| HTTP POST | `/api/v1/login` | 登录，返回 JWT + 节点地址 |
| WebSocket | `ws://{node}/ws?token={jwt}` | IM 连接 |
| WS Message | `{"type":"msg","to":2,"content":"hello","msg_id":"uuid"}` | 发送消息 |

## 测试

```bash
go test ./... -v
```
