# Seckill - 高并发秒杀系统

基于 Go + Redis + Kafka + MySQL 的秒杀系统，支持 10,000+ QPS。

## 技术栈

| 层 | 选型 | 说明 |
|---|---|---|
| 语言 | Go 1.24 | 高性能 |
| Web | Gin | 轻量 |
| 数据库 | MySQL 8.0 | 订单持久化 |
| 缓存 | Redis 7 | 库存预热 + Lua 原子扣减 |
| 消息队列 | Kafka | 异步下单削峰 |
| ORM | GORM | 仅订单写入 |

## 快速启动

```bash
docker compose up -d
curl http://localhost:8081/api/v1/ping
```

## 核心流程

```
用户请求 → 限流(5/s) → Redis Lua 原子扣库存 → Kafka 异步投递 → Consumer 写 MySQL
                          │
                     ┌────┴────┐
                   成功      3种失败情况
                   │        (sold_out / already_bought / rate_limited)
                   ▼
             Kafka Producer
             (不阻塞响应)
```

## API

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/seckill/sessions` | 创建秒杀场次 |
| GET | `/seckill/sessions` | 场次列表 |
| POST | `/seckill/items` | 创建秒杀商品 |
| GET | `/seckill/items` | 商品列表 |
| POST | `/seckill/items/:id/preheat` | 库存预热到 Redis |
| POST | `/seckill/buy` | 执行秒杀 |
| GET | `/seckill/result/:id` | 查询结果 |

## 压测

```bash
wrk -t12 -c400 -d30s -s scripts/bench/seckill.lua \
  http://localhost:8081/api/v1/seckill/buy
```

## 对账

```bash
./scripts/reconcile.sh
# Redis stock + MySQL orders = total stock
```
