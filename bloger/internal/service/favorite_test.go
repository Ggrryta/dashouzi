package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"bloger/internal/model"
)

type mockFavoriteRepo struct {
	favorites map[string]*model.Favorite
	nextID    uint
}

func newMockFavoriteRepo() *mockFavoriteRepo {
	return &mockFavoriteRepo{favorites: make(map[string]*model.Favorite), nextID: 1}
}

func (m *mockFavoriteRepo) Exists(_ context.Context, userID, articleID uint) (bool, error) {
	key := favKey(userID, articleID)
	_, ok := m.favorites[key]
	return ok, nil
}

func (m *mockFavoriteRepo) Create(_ context.Context, f *model.Favorite) error {
	f.ID = m.nextID
	m.nextID++
	key := favKey(f.UserID, f.ArticleID)
	m.favorites[key] = f
	return nil
}

func (m *mockFavoriteRepo) Delete(_ context.Context, userID, articleID uint) error {
	key := favKey(userID, articleID)
	delete(m.favorites, key)
	return nil
}

func (m *mockFavoriteRepo) List(_ context.Context, userID uint, page, size int) ([]*model.Favorite, int64, error) {
	var result []*model.Favorite
	for _, f := range m.favorites {
		if f.UserID == userID {
			result = append(result, f)
		}
	}
	return result, int64(len(result)), nil
}

func favKey(userID, articleID uint) string {
	return string(rune(userID)) + "-" + string(rune(articleID))
}

func TestFavoriteToggle_FirstFav_Created(t *testing.T) {
	repo := newMockFavoriteRepo()
	svc := NewFavoriteService(repo)

	faved, err := svc.Toggle(context.Background(), 1, 1)
	assert.NoError(t, err)
	assert.True(t, faved)
}

func TestFavoriteToggle_SecondFav_Removed(t *testing.T) {
	repo := newMockFavoriteRepo()
	svc := NewFavoriteService(repo)

	svc.Toggle(context.Background(), 1, 1)
	faved, _ := svc.Toggle(context.Background(), 1, 1)

	assert.False(t, faved)
}

func TestFavoriteList_Success(t *testing.T) {
	repo := newMockFavoriteRepo()
	svc := NewFavoriteService(repo)

	svc.Toggle(context.Background(), 1, 1)
	svc.Toggle(context.Background(), 1, 2)

	list, total, err := svc.List(context.Background(), 1, 1, 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, list, 2)
}
