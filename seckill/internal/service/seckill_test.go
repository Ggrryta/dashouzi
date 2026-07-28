package service

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"seckill/internal/repository"
)

// mockRedisSeckill 模拟 Redis 执行 Lua 脚本 / Set / Get
type mockRedisSeckill struct {
	mu   sync.Mutex
	data map[string]int
	sets map[string]map[int]bool
}

func newMockRedisSeckill() *mockRedisSeckill {
	return &mockRedisSeckill{
		data: make(map[string]int),
		sets: make(map[string]map[int]bool),
	}
}

func (m *mockRedisSeckill) Set(_ context.Context, _ string, _ interface{}, _ time.Duration) error {
	return nil
}

func (m *mockRedisSeckill) Get(_ context.Context, key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return strconv.Itoa(m.data[key]), nil
}

func (m *mockRedisSeckill) Del(_ context.Context, _ ...string) error {
	return nil
}

// Eval 模拟 Lua 脚本执行（线程安全）
func (m *mockRedisSeckill) Eval(_ context.Context, _ string, keys []string, args ...interface{}) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	stockKey := keys[0]
	boughtKey := keys[1]
	userID := args[0].(string)

	// 检查已购买
	if m.sets[boughtKey] != nil && m.sets[boughtKey][atoi(userID)] {
		return -1, nil
	}

	// 检查库存
	stock := m.data[stockKey]
	if stock <= 0 {
		return 0, nil
	}

	// 扣库存 + 记录已购
	m.data[stockKey] = stock - 1
	if m.sets[boughtKey] == nil {
		m.sets[boughtKey] = make(map[int]bool)
	}
	m.sets[boughtKey][atoi(userID)] = true

	return 1, nil
}

func (m *mockRedisSeckill) SIsMember(_ context.Context, key string, member interface{}) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	set := m.sets[key]
	if set == nil {
		return false, nil
	}
	return set[member.(int)], nil
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// 确保实现接口
var _ repository.RedisCmdClient = (*mockRedisSeckill)(nil)

// ====== Tests ======

func TestSeckill_Success(t *testing.T) {
	redis := newMockRedisSeckill()
	redis.data["seckill:stock:1"] = 100
	svc := NewSeckillService(redis, nil)

	result, err := svc.Execute(context.Background(), 1, 1)
	assert.NoError(t, err)
	assert.Equal(t, ResultSuccess, result)

	val, _ := redis.Get(context.Background(), "seckill:stock:1")
	assert.Equal(t, "99", val)

	bought, _ := redis.SIsMember(context.Background(), "seckill:bought:1", 1)
	assert.True(t, bought)
}

func TestSeckill_SoldOut(t *testing.T) {
	redis := newMockRedisSeckill()
	redis.data["seckill:stock:1"] = 0
	svc := NewSeckillService(redis, nil)

	result, _ := svc.Execute(context.Background(), 1, 1)
	assert.Equal(t, ResultSoldOut, result)
}

func TestSeckill_AlreadyBought(t *testing.T) {
	redis := newMockRedisSeckill()
	redis.data["seckill:stock:1"] = 100
	redis.sets["seckill:bought:1"] = map[int]bool{1: true}
	svc := NewSeckillService(redis, nil)

	result, _ := svc.Execute(context.Background(), 1, 1)
	assert.Equal(t, ResultAlreadyBought, result)
}

func TestSeckill_ConcurrentNoOversell(t *testing.T) {
	redis := newMockRedisSeckill()
	initialStock := 50
	redis.data["seckill:stock:1"] = initialStock
	svc := NewSeckillService(redis, nil)

	var wg sync.WaitGroup
	success := atomic.Int64{}
	total := 100

	for i := 0; i < total; i++ {
		wg.Add(1)
		go func(uid int) {
			defer wg.Done()
			result, _ := svc.Execute(context.Background(), uint(uid), 1)
			if result == ResultSuccess {
				success.Add(1)
			}
		}(i)
	}
	wg.Wait()

	val, _ := redis.Get(context.Background(), "seckill:stock:1")
	remaining, _ := strconv.Atoi(val)
	sold := initialStock - remaining

	// 关键断言：实际售出 = 成功次数，且不超过初始库存
	assert.Equal(t, int(success.Load()), sold, "success count must match actual sold")
	assert.LessOrEqual(t, sold, initialStock, "must not oversell")
}

func TestSeckill_OnePersonOneOrder(t *testing.T) {
	redis := newMockRedisSeckill()
	redis.data["seckill:stock:1"] = 100
	svc := NewSeckillService(redis, nil)

	// 同一用户抢 3 次
	svc.Execute(context.Background(), 1, 1) // 成功
	svc.Execute(context.Background(), 1, 1) // 已买
	result, _ := svc.Execute(context.Background(), 1, 1) // 已买

	assert.Equal(t, ResultAlreadyBought, result)
	val, _ := redis.Get(context.Background(), "seckill:stock:1")
	assert.Equal(t, "99", val) // 只扣了 1 件
}
