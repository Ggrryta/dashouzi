package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"bloger/internal/model"
)

type mockLikeRepo struct {
	likes map[string]*model.Like
}

func newMockLikeRepo() *mockLikeRepo {
	return &mockLikeRepo{likes: make(map[string]*model.Like)}
}

func (m *mockLikeRepo) Exists(_ context.Context, userID uint, targetType string, targetID uint) (bool, error) {
	key := likeKey(userID, targetType, targetID)
	_, ok := m.likes[key]
	return ok, nil
}

func (m *mockLikeRepo) Create(_ context.Context, like *model.Like) error {
	key := likeKey(like.UserID, like.TargetType, like.TargetID)
	m.likes[key] = like
	return nil
}

func (m *mockLikeRepo) Delete(_ context.Context, userID uint, targetType string, targetID uint) error {
	key := likeKey(userID, targetType, targetID)
	delete(m.likes, key)
	return nil
}

func likeKey(userID uint, targetType string, targetID uint) string {
	return string(rune(userID)) + targetType + string(rune(targetID))
}

func TestLikeToggle_FirstLike_Created(t *testing.T) {
	repo := newMockLikeRepo()
	svc := NewLikeService(repo)

	liked, err := svc.Toggle(context.Background(), 1, "article", 1)
	assert.NoError(t, err)
	assert.True(t, liked)
}

func TestLikeToggle_SecondLike_Removed(t *testing.T) {
	repo := newMockLikeRepo()
	svc := NewLikeService(repo)

	// 第一次
	liked, _ := svc.Toggle(context.Background(), 1, "article", 1)
	assert.True(t, liked)

	// 第二次
	liked, err := svc.Toggle(context.Background(), 1, "article", 1)
	assert.NoError(t, err)
	assert.False(t, liked)
}

func TestLikeToggle_ThirdLike_CreatedAgain(t *testing.T) {
	repo := newMockLikeRepo()
	svc := NewLikeService(repo)

	svc.Toggle(context.Background(), 1, "article", 1) // created
	svc.Toggle(context.Background(), 1, "article", 1) // removed
	liked, err := svc.Toggle(context.Background(), 1, "article", 1) // created again

	assert.NoError(t, err)
	assert.True(t, liked)
}

func TestLikeToggle_DifferentTargets(t *testing.T) {
	repo := newMockLikeRepo()
	svc := NewLikeService(repo)

	svc.Toggle(context.Background(), 1, "article", 1)
	liked, _ := svc.Toggle(context.Background(), 1, "comment", 1)

	assert.True(t, liked) // 不同 target_type，独立
}

func TestIsLiked_True(t *testing.T) {
	repo := newMockLikeRepo()
	svc := NewLikeService(repo)

	svc.Toggle(context.Background(), 1, "article", 1)
	liked, err := svc.IsLiked(context.Background(), 1, "article", 1)

	assert.NoError(t, err)
	assert.True(t, liked)
}

func TestIsLiked_False(t *testing.T) {
	repo := newMockLikeRepo()
	svc := NewLikeService(repo)

	liked, err := svc.IsLiked(context.Background(), 1, "article", 999)
	assert.NoError(t, err)
	assert.False(t, liked)
}
