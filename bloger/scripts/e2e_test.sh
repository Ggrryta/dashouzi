#!/bin/bash
# 博客系统 E2E 全链路测试
# ./scripts/e2e_test.sh

BASE="http://localhost:8080/api/v1"
PASS=0
FAIL=0

check() {
    local desc="$1" expected="$2" actual="$3"
    if echo "$actual" | grep -q "$expected"; then
        echo "  ✅ $desc"
        ((PASS++))
    else
        echo "  ❌ $desc"
        echo "     expected: $expected"
        echo "     got:      $(echo "$actual" | head -c 200)"
        ((FAIL++))
    fi
}

echo "=========================================="
echo "  博客系统 E2E 全链路测试"
echo "=========================================="

# ====== 1. 健康检查 ======
echo ""
echo "--- 1. 健康检查 ---"
check "ping" '"code":0' "$(curl -s $BASE/ping)"

# ====== 2. 用户注册 ======
echo ""
echo "--- 2. 用户注册 ---"
EMAIL="e2e_$(date +%s)@blog.com"
check "注册" '"code":0' "$(curl -s -X POST $BASE/users/register -H 'Content-Type: application/json' -d "{\"username\":\"e2e_user\",\"email\":\"$EMAIL\",\"password\":\"E2eTest123\"}")"
check "重复注册" '"code":20002' "$(curl -s -X POST $BASE/users/register -H 'Content-Type: application/json' -d "{\"username\":\"e2e_user\",\"email\":\"$EMAIL\",\"password\":\"E2eTest123\"}")"

# ====== 3. 用户登录 ======
echo ""
echo "--- 3. 用户登录 ---"
LOGIN_RESP=$(curl -s -X POST $BASE/users/login -H 'Content-Type: application/json' -d "{\"email\":\"$EMAIL\",\"password\":\"E2eTest123\"}")
check "登录成功" '"code":0' "$LOGIN_RESP"
TOKEN=$(echo "$LOGIN_RESP" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)

check "错误密码" '"code":20001' "$(curl -s -X POST $BASE/users/login -H 'Content-Type: application/json' -d "{\"email\":\"$EMAIL\",\"password\":\"wrong\"}")"

# ====== 4. 个人信息 ======
echo ""
echo "--- 4. 个人信息 ---"
check "获取信息" '"username":"e2e_user"' "$(curl -s $BASE/users/me -H "Authorization: Bearer $TOKEN")"
check "无token" '"code":20006' "$(curl -s $BASE/users/me)"

# ====== 5. 文章 CRUD ======
echo ""
echo "--- 5. 文章 CRUD ---"
CREATE_RESP=$(curl -s -X POST $BASE/articles -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d '{"title":"E2E测试","content":"端到端测试","tags":["e2e"]}')
check "创建文章" '"status":"draft"' "$CREATE_RESP"
AID=$(echo "$CREATE_RESP" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)

check "提审" '"status":"reviewing"' "$(curl -s -X PATCH $BASE/articles/$AID/status -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d '{"status":"reviewing"}')"
check "发布" '"status":"published"' "$(curl -s -X PATCH $BASE/articles/$AID/status -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d '{"status":"published"}')"
check "列表可见" '"total"' "$(curl -s $BASE/articles)"
check "非法流转" '"code":30002' "$(curl -s -X PATCH $BASE/articles/$AID/status -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d '{"status":"draft"}')"

# ====== 6. 评论 ======
echo ""
echo "--- 6. 评论 ---"
CMT_RESP=$(curl -s -X POST $BASE/comments -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d "{\"article_id\":$AID,\"content\":\"好文章！\"}")
check "发表评论" '"code":0' "$CMT_RESP"
CID=$(echo "$CMT_RESP" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)

check "回复评论" '"parent_id":'"$CID" "$(curl -s -X POST $BASE/comments -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d "{\"article_id\":$AID,\"parent_id\":$CID,\"content\":\"同意\"}")"
check "评论树" '"replies"' "$(curl -s $BASE/articles/$AID/comments)"

# ====== 7. 点赞 ======
echo ""
echo "--- 7. 点赞 ---"
check "点赞(创建)" '"liked":true' "$(curl -s -X POST $BASE/likes -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d "{\"target_type\":\"article\",\"target_id\":$AID}")"
check "点赞(取消)" '"liked":false' "$(curl -s -X POST $BASE/likes -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d "{\"target_type\":\"article\",\"target_id\":$AID}")"

# ====== 8. 收藏 ======
echo ""
echo "--- 8. 收藏 ---"
check "收藏" '"favorited":true' "$(curl -s -X POST $BASE/favorites -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d "{\"article_id\":$AID}")"
check "收藏列表" '"total":1' "$(curl -s $BASE/favorites -H "Authorization: Bearer $TOKEN")"

# ====== 9. 搜索 ======
echo ""
echo "--- 9. 搜索 ---"
check "搜索有结果" '"total":1' "$(curl -s "$BASE/search?q=E2E")"
check "搜索无结果" '"total":0' "$(curl -s "$BASE/search?q=nonexistent")"

# ====== 10. 统计 ======
echo ""
echo "--- 10. 统计 ---"
check "热门" '"articles"' "$(curl -s $BASE/stats/trending)"
check "排行" '"ranking"' "$(curl -s $BASE/stats/users)"

# ====== 11. 限流 ======
echo ""
echo "--- 11. 限流 ---"
HIT=0
for i in $(seq 1 35); do
    CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/users/login -H 'Content-Type: application/json' -d '{"email":"x@x.com","password":"x"}')
    if [ "$CODE" = "429" ]; then
        echo "  ✅ 第 $i 次触发限流 429"
        HIT=1
        break
    fi
done
if [ "$HIT" = "0" ]; then
    echo "  ❌ 限流未触发"
    ((FAIL++))
else
    ((PASS++))
fi

# ====== 结果 ======
echo ""
echo "=========================================="
echo "  通过 $PASS / 失败 $FAIL"
echo "=========================================="
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
