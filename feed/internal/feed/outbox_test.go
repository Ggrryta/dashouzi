package feed

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// T1.7: 发件箱上限裁剪
func TestOutbox_TrimToLimit(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	outbox := NewOutbox(rdb, 500)
	ctx := context.Background()

	// 写入 600 条
	for i := int64(0); i < 600; i++ {
		err := outbox.Add(ctx, 1, i, float64(i))
		assert.NoError(t, err)
	}

	count, err := outbox.Count(ctx, 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(500), count, "should trim to 500")

	// 验证保留的是最近 500 条（score 100-599，最小的在索引0）
	posts, err := outbox.GetRange(ctx, 1, 0, 0)
	assert.NoError(t, err)
	assert.Equal(t, int64(100), posts[0], "oldest kept should have score=100")
}

// 发件箱基本读写
func TestOutbox_AddAndGetRange(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	outbox := NewOutbox(rdb, 100)
	ctx := context.Background()

	outbox.Add(ctx, 1, 100, 1000.0)
	outbox.Add(ctx, 1, 200, 2000.0)
	outbox.Add(ctx, 1, 300, 3000.0)

	posts, err := outbox.GetRange(ctx, 1, 0, -1)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(posts))
	// Sorted Set 按 score 排序，0→-1 从小到大
	assert.Equal(t, int64(100), posts[0])
	assert.Equal(t, int64(300), posts[2])
}

// 空发件箱
func TestOutbox_Empty(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	outbox := NewOutbox(rdb, 100)
	count, err := outbox.Count(context.Background(), 999)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)
}
