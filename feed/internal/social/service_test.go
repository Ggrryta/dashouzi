package social

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---------- fake repos for unit tests ----------

type fakeSocialRepo struct {
	follows   map[string]bool
	fanCounts map[int64]int
	bigVUsers map[int64]bool
}

func newFakeSocialRepo() *fakeSocialRepo {
	return &fakeSocialRepo{
		follows:   make(map[string]bool),
		fanCounts: make(map[int64]int),
		bigVUsers: make(map[int64]bool),
	}
}

func (r *fakeSocialRepo) AddFollow(ctx context.Context, followerID, followeeID int64) error {
	key := fmt.Sprintf("%d:%d", followerID, followeeID)
	r.follows[key] = true
	r.fanCounts[followeeID]++
	return nil
}

func (r *fakeSocialRepo) RemoveFollow(ctx context.Context, followerID, followeeID int64) error {
	key := fmt.Sprintf("%d:%d", followerID, followeeID)
	if r.follows[key] {
		r.follows[key] = false
		r.fanCounts[followeeID]--
	}
	return nil
}

func (r *fakeSocialRepo) IsFollowing(ctx context.Context, followerID, followeeID int64) bool {
	return r.follows[fmt.Sprintf("%d:%d", followerID, followeeID)]
}

func (r *fakeSocialRepo) FollowerCount(ctx context.Context, userID int64) int {
	return r.fanCounts[userID]
}

func (r *fakeSocialRepo) GetFollowers(ctx context.Context, userID int64) ([]int64, error) {
	var fans []int64
	for key, active := range r.follows {
		if !active {
			continue
		}
		parts := strings.Split(key, ":")
		fid, _ := strconv.ParseInt(parts[0], 10, 64)
		foid, _ := strconv.ParseInt(parts[1], 10, 64)
		if foid == userID {
			fans = append(fans, fid)
		}
	}
	return fans, nil
}

func (r *fakeSocialRepo) GetFollowing(ctx context.Context, userID int64) ([]int64, error) {
	var follows []int64
	for key, active := range r.follows {
		if !active {
			continue
		}
		parts := strings.Split(key, ":")
		fid, _ := strconv.ParseInt(parts[0], 10, 64)
		foid, _ := strconv.ParseInt(parts[1], 10, 64)
		if fid == userID {
			follows = append(follows, foid)
		}
	}
	return follows, nil
}

func (r *fakeSocialRepo) SetBigV(ctx context.Context, userID int64, isBigV bool) error {
	r.bigVUsers[userID] = isBigV
	return nil
}

func (r *fakeSocialRepo) IsBigVUser(ctx context.Context, userID int64) bool {
	return r.bigVUsers[userID]
}

// ---------- T2.4: 关注自己返回错误 ----------

func TestFollow_SelfFollow(t *testing.T) {
	svc := NewService(newFakeSocialRepo(), nil, 100000)
	err := svc.Follow(context.Background(), 1, 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "self")
}

// ---------- T2.5-T2.7: 大V判定 ----------

func TestIsBigV_AboveThreshold(t *testing.T) {
	repo := newFakeSocialRepo()
	repo.fanCounts[100] = 100000
	checker := NewBigVChecker(repo, 100000)
	assert.True(t, checker.IsBigV(context.Background(), 100))
}

func TestIsBigV_BelowThreshold(t *testing.T) {
	repo := newFakeSocialRepo()
	repo.fanCounts[200] = 99999
	checker := NewBigVChecker(repo, 100000)
	assert.False(t, checker.IsBigV(context.Background(), 200))
}

func TestIsBigV_Boundary(t *testing.T) {
	repo := newFakeSocialRepo()
	repo.fanCounts[300] = 99999
	checker := NewBigVChecker(repo, 100000)
	assert.False(t, checker.IsBigV(context.Background(), 300))

	repo2 := newFakeSocialRepo()
	repo2.fanCounts[400] = 100000
	checker2 := NewBigVChecker(repo2, 100000)
	assert.True(t, checker2.IsBigV(context.Background(), 400))
}
