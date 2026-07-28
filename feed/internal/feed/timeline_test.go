package feed

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// T2.10: 收件箱上限裁剪
func TestTimeline_TrimToLimit(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	tl := NewTimeline(rdb, 1000)
	ctx := context.Background()

	// 写入 1200 条
	for i := int64(0); i < 1200; i++ {
		err := tl.AddPost(ctx, 1, i, float64(i))
		assert.NoError(t, err)
	}

	count, err := tl.Count(ctx, 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(1000), count)

	// 最老的 200 条（score 0-199）被裁掉，保留 score 200-1199
	// GetRecent 返回最新的：1199
	posts, err := tl.GetRecent(ctx, 1, 1, 0)
	assert.NoError(t, err)
	assert.Equal(t, int64(1199), posts[0], "newest kept should have score=1199")
}

// 时间线基本写入读取
func TestTimeline_AddAndGetRecent(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	tl := NewTimeline(rdb, 100)
	ctx := context.Background()

	tl.AddPost(ctx, 1, 100, 1000.0)
	tl.AddPost(ctx, 1, 200, 2000.0)
	tl.AddPost(ctx, 1, 300, 3000.0)

	// GetRecent 返回 score 最高的 N 条（最新的）
	posts, err := tl.GetRecent(ctx, 1, 2, 0)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(posts))
	// 最新的在前：score 3000, 2000
	assert.Equal(t, int64(300), posts[0])
	assert.Equal(t, int64(200), posts[1])
}

// 空时间线
func TestTimeline_Empty(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	tl := NewTimeline(rdb, 100)
	posts, err := tl.GetRecent(context.Background(), 999, 10, 0)
	assert.NoError(t, err)
	assert.Empty(t, posts)
}
