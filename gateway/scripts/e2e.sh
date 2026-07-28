#!/bin/bash
GW="http://localhost:9000"
PASS=0; FAIL=0
check() { if echo "$3" | grep -q "$2"; then echo "  ✅ $1"; ((PASS++)); else echo "  ❌ $1"; ((FAIL++)); fi; }

echo "=== Gateway E2E ==="
check "health" '"status":"ok"' "$(curl -s $GW/health)"
check "blog ping" '"data":"pong"' "$(curl -s $GW/blog/api/v1/ping)"
check "no route 404" "404" "$(curl -s -o /dev/null -w '%{http_code}' $GW/unknown)"

echo "=== Auth ==="
check "no token 401" "401" "$(curl -s -o /dev/null -w '%{http_code}' $GW/blog/api/v1/users/me)"

echo "=== RateLimit ==="
for i in $(seq 1 7); do
  code=$(curl -s -o /dev/null -w '%{http_code}' $GW/blog/api/v1/ping)
  if [ "$code" = "429" ]; then echo "  ✅ rate limit 429 at req $i"; ((PASS++)); break; fi
done

echo "=== Pass:$PASS Fail:$FAIL ==="
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
