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
	_, ok := m.likes[likeKey(userID, targetType, targetID)]
	return ok, nil
}

func (m *mockLikeRepo) Create(_ context.Context, like *model.Like) error {
	m.likes[likeKey(like.UserID, like.TargetType, like.TargetID)] = like
	return nil
}

func (m *mockLikeRepo) Delete(_ context.Context, userID uint, targetType string, targetID uint) error {
	delete(m.likes, likeKey(userID, targetType, targetID))
	return nil
}

// Toggle 模拟 repo 层原子切换(map 操作非真并发,但逻辑等价)。
func (m *mockLikeRepo) Toggle(_ context.Context, userID uint, targetType string, targetID uint) (bool, error) {
	key := likeKey(userID, targetType, targetID)
	if _, ok := m.likes[key]; ok {
		delete(m.likes, key)
		return false, nil
	}
	m.likes[key] = &model.Like{UserID: userID, TargetType: targetType, TargetID: targetID}
	return true, nil
}

func likeKey(userID uint, targetType string, targetID uint) string {
	return string(rune(userID)) + targetType + string(rune(targetID))
}

// newLikeSvcWithTargets 构造带 article(1)/comment(1) 目标的 LikeService。
func newLikeSvcWithTargets() (*LikeService, *mockLikeRepo) {
	likeRepo := newMockLikeRepo()
	articleRepo := newMockArticleRepo()
	commentRepo := newMockCommentRepo()
	_ = articleRepo.Create(context.Background(), &model.Article{Title: "a", Status: "published"})
	_ = commentRepo.Create(context.Background(), &model.Comment{ArticleID: 1, Content: "c"})
	return NewLikeService(likeRepo, articleRepo, commentRepo), likeRepo
}

func TestLikeToggle_FirstLike_Created(t *testing.T) {
	svc, _ := newLikeSvcWithTargets()

	liked, err := svc.Toggle(context.Background(), 1, "article", 1)
	assert.NoError(t, err)
	assert.True(t, liked)
}

func TestLikeToggle_SecondLike_Removed(t *testing.T) {
	svc, _ := newLikeSvcWithTargets()

	liked, _ := svc.Toggle(context.Background(), 1, "article", 1)
	assert.True(t, liked)

	liked, err := svc.Toggle(context.Background(), 1, "article", 1)
	assert.NoError(t, err)
	assert.False(t, liked)
}

func TestLikeToggle_ThirdLike_CreatedAgain(t *testing.T) {
	svc, _ := newLikeSvcWithTargets()

	svc.Toggle(context.Background(), 1, "article", 1) // created
	svc.Toggle(context.Background(), 1, "article", 1) // removed
	liked, err := svc.Toggle(context.Background(), 1, "article", 1) // created again

	assert.NoError(t, err)
	assert.True(t, liked)
}

func TestLikeToggle_DifferentTargets(t *testing.T) {
	svc, _ := newLikeSvcWithTargets()

	svc.Toggle(context.Background(), 1, "article", 1)
	liked, _ := svc.Toggle(context.Background(), 1, "comment", 1)

	assert.True(t, liked) // 不同 target_type,独立
}

func TestLikeToggle_TargetNotFound(t *testing.T) {
	svc, _ := newLikeSvcWithTargets()

	_, err := svc.Toggle(context.Background(), 1, "article", 999) // 不存在的文章
	assert.ErrorIs(t, err, ErrTargetNotFound)
}

func TestIsLiked_True(t *testing.T) {
	svc, _ := newLikeSvcWithTargets()

	svc.Toggle(context.Background(), 1, "article", 1)
	liked, err := svc.IsLiked(context.Background(), 1, "article", 1)

	assert.NoError(t, err)
	assert.True(t, liked)
}

func TestIsLiked_False(t *testing.T) {
	svc, _ := newLikeSvcWithTargets()

	liked, err := svc.IsLiked(context.Background(), 1, "article", 999)
	assert.NoError(t, err)
	assert.False(t, liked)
}
