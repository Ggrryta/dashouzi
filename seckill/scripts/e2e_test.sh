#!/bin/bash
# 秒杀系统 E2E 全链路测试

BASE="http://localhost:8081/api/v1/seckill"
PASS=0
FAIL=0

check() {
    local desc="$1" expected="$2" actual="$3"
    if echo "$actual" | grep -q "$expected"; then
        echo "  ✅ $desc"
        ((PASS++))
    else
        echo "  ❌ $desc (expected: $expected)"
        ((FAIL++))
    fi
}

echo "=========================================="
echo "  Seckill E2E Test"
echo "=========================================="

# 1. Ping
echo "" && echo "--- Ping ---"
check "health" '"code":0' "$(curl -s $BASE/../ping)"

# 2. 预热
echo "" && echo "--- Preheat ---"
curl -s -X POST $BASE/items/1/preheat > /dev/null
STOCK=$(curl -s $BASE/result/1 -H "X-User-Id: 1" | grep -o '"stock":[0-9]*' | cut -d: -f2)
check "预热成功" "[1-9]" "$STOCK"

# 3. 秒杀
echo "" && echo "--- Seckill ---"
R=$(curl -s -X POST $BASE/buy -H "X-User-Id: 100" -H "Content-Type: application/json" -d '{"item_id":1}')
check "抢购成功" '"result":"success"' "$R"

R=$(curl -s -X POST $BASE/buy -H "X-User-Id: 100" -H "Content-Type: application/json" -d '{"item_id":1}')
check "重复购买" '"code":50002' "$R"

# 4. 库存不足
echo "" && echo "--- Sold Out ---"
R=$(curl -s -X POST $BASE/buy -H "X-User-Id: 999" -H "Content-Type: application/json" -d '{"item_id":999}')
check "不存在商品" '"code"' "$R"

# 5. 对账
echo "" && echo "--- Result ---"
R=$(curl -s $BASE/result/1 -H "X-User-Id: 100")
check "已购买标记" '"bought":true' "$R"

echo ""
echo "=========================================="
echo "  Pass: $PASS / Fail: $FAIL"
echo "=========================================="
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
