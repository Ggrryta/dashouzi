#!/bin/bash
# 对账脚本：比较 Redis 库存 + MySQL 订单数 = 初始库存

echo "=== Seckill Reconciliation ==="

REDIS_HOST="localhost"
REDIS_PORT="6380"
MYSQL_HOST="localhost"
MYSQL_PORT="3307"

# Redis 剩余库存
STOCK=$(redis-cli -h $REDIS_HOST -p $REDIS_PORT GET seckill:stock:1 2>/dev/null)
STOCK=${STOCK:-0}
echo "Redis remaining stock: $STOCK"

# Redis 已购买人数
BOUGHT=$(redis-cli -h $REDIS_HOST -p $REDIS_PORT SCARD seckill:bought:1 2>/dev/null)
BOUGHT=${BOUGHT:-0}
echo "Redis unique buyers:   $BOUGHT"

# MySQL 订单数
ORDERS=$(mysql -h $MYSQL_HOST -P $MYSQL_PORT -u seckill -pseckill123 seckill -N -e \
    "SELECT COUNT(*) FROM seckill_orders WHERE item_id=1" 2>/dev/null)
ORDERS=${ORDERS:-0}
echo "MySQL order count:    $ORDERS"

# 对账：Redis stock + sold = 初始库存?
# 初始库存从 MySQL 查
TOTAL=$(mysql -h $MYSQL_HOST -P $MYSQL_PORT -u seckill -pseckill123 seckill -N -e \
    "SELECT total_stock FROM seckill_items WHERE id=1" 2>/dev/null)
TOTAL=${TOTAL:-100}
echo "Initial total stock:  $TOTAL"

echo ""
echo "--- Verification ---"
SOLD=$((TOTAL - STOCK))
echo "Calculated sold:      $SOLD"
echo "Redis buyers:         $BOUGHT"
echo "MySQL orders:         $ORDERS"

if [ "$BOUGHT" = "$ORDERS" ] && [ "$SOLD" -eq "$ORDERS" ]; then
    echo "✅ ALL MATCH — no oversell"
else
    echo "❌ MISMATCH detected!"
fi
