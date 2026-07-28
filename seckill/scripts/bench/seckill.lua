-- wrk 压测脚本：多用户并发秒杀
-- 用法: wrk -t12 -c400 -d30s -s scripts/bench/seckill.lua http://localhost:8081/api/v1/seckill/buy

counter = 0

request = function()
    counter = counter + 1
    local user_id = counter
    local body = string.format('{"item_id":%d}', (counter % 2) + 1) -- 轮流抢商品1和2

    return wrk.format("POST", nil, {
        ["X-User-Id"] = tostring(user_id),
        ["Content-Type"] = "application/json"
    }, body)
end

response = function(status, headers, body)
    if status == 200 then
        -- 成功购买
    elseif status == 429 then
        -- 限流
    end
end
