package feed

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// fakeFollowerProvider 假粉丝提供者
type fakeFollowerProvider struct {
	followers map[int64][]int64 // userID -> followerIDs
	bigVs     map[int64]bool
}

func (f *fakeFollowerProvider) GetFollowers(ctx context.Context, userID int64) ([]int64, error) {
	return f.followers[userID], nil
}

func (f *fakeFollowerProvider) IsBigV(ctx context.Context, userID int64) bool {
	return f.bigVs[userID]
}

// T2.8: 普通用户发帖 → 扩散到粉丝收件箱
func TestDiffusion_NormalUser_SpreadsToFollowers(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	timeline := NewTimeline(rdb, 1000)
	follows := &fakeFollowerProvider{
		followers: map[int64][]int64{
			1: {10, 20, 30}, // 用户1有3个粉丝
		},
		bigVs: map[int64]bool{1: false}, // 用户1不是大V
	}
	diffusion := NewDiffusion(follows, timeline)

	n, err := diffusion.Spread(context.Background(), 1, 100, 1000.0)
	assert.NoError(t, err)
	assert.Equal(t, 3, n)

	// 验证每个粉丝收件箱都有帖子
	for _, fid := range []int64{10, 20, 30} {
		posts, err := timeline.GetRecent(context.Background(), fid, 10, 0)
		assert.NoError(t, err)
		assert.Contains(t, posts, int64(100))
	}
}

// T2.9: 大V 发帖 → 不扩散到粉丝收件箱
func TestDiffusion_BigV_DoesNotSpread(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	timeline := NewTimeline(rdb, 1000)
	follows := &fakeFollowerProvider{
		followers: map[int64][]int64{
			1: {10, 20, 30},
		},
		bigVs: map[int64]bool{1: true}, // 用户1是大V
	}
	diffusion := NewDiffusion(follows, timeline)

	n, err := diffusion.Spread(context.Background(), 1, 100, 1000.0)
	assert.NoError(t, err)
	assert.Equal(t, 0, n) // 扩散数为 0

	// 验证粉丝收件箱为空
	for _, fid := range []int64{10, 20, 30} {
		posts, err := timeline.GetRecent(context.Background(), fid, 10, 0)
		assert.NoError(t, err)
		assert.Empty(t, posts)
	}

	// 但发件箱应该有帖子（这个由 Service 负责）
}

// 无粉丝场景
func TestDiffusion_NoFollowers(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	timeline := NewTimeline(rdb, 1000)
	follows := &fakeFollowerProvider{
		followers: map[int64][]int64{1: nil},
		bigVs:     map[int64]bool{1: false},
	}
	diffusion := NewDiffusion(follows, timeline)

	n, err := diffusion.Spread(context.Background(), 1, 100, 1000.0)
	assert.NoError(t, err)
	assert.Equal(t, 0, n)
}
