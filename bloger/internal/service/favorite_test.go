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
	_, ok := m.favorites[favKey(userID, articleID)]
	return ok, nil
}

func (m *mockFavoriteRepo) Create(_ context.Context, f *model.Favorite) error {
	f.ID = m.nextID
	m.nextID++
	m.favorites[favKey(f.UserID, f.ArticleID)] = f
	return nil
}

func (m *mockFavoriteRepo) Delete(_ context.Context, userID, articleID uint) error {
	delete(m.favorites, favKey(userID, articleID))
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

func (m *mockFavoriteRepo) Toggle(_ context.Context, userID, articleID uint) (bool, error) {
	key := favKey(userID, articleID)
	if _, ok := m.favorites[key]; ok {
		delete(m.favorites, key)
		return false, nil
	}
	m.favorites[key] = &model.Favorite{UserID: userID, ArticleID: articleID, ID: m.nextID}
	m.nextID++
	return true, nil
}

func favKey(userID, articleID uint) string {
	return string(rune(userID)) + "-" + string(rune(articleID))
}

// newFavSvcWithArticles 构造带 article(1)/article(2) 的 FavoriteService。
func newFavSvcWithArticles() (*FavoriteService, *mockFavoriteRepo) {
	favRepo := newMockFavoriteRepo()
	articleRepo := newMockArticleRepo()
	_ = articleRepo.Create(context.Background(), &model.Article{Title: "a1", Status: "published"})
	_ = articleRepo.Create(context.Background(), &model.Article{Title: "a2", Status: "published"})
	return NewFavoriteService(favRepo, articleRepo), favRepo
}

func TestFavoriteToggle_FirstFav_Created(t *testing.T) {
	svc, _ := newFavSvcWithArticles()

	faved, err := svc.Toggle(context.Background(), 1, 1)
	assert.NoError(t, err)
	assert.True(t, faved)
}

func TestFavoriteToggle_SecondFav_Removed(t *testing.T) {
	svc, _ := newFavSvcWithArticles()

	svc.Toggle(context.Background(), 1, 1)
	faved, _ := svc.Toggle(context.Background(), 1, 1)

	assert.False(t, faved)
}

func TestFavoriteToggle_TargetNotFound(t *testing.T) {
	svc, _ := newFavSvcWithArticles()

	_, err := svc.Toggle(context.Background(), 1, 999)
	assert.ErrorIs(t, err, ErrTargetNotFound)
}

func TestFavoriteList_Success(t *testing.T) {
	svc, _ := newFavSvcWithArticles()

	svc.Toggle(context.Background(), 1, 1)
	svc.Toggle(context.Background(), 1, 2)

	list, total, err := svc.List(context.Background(), 1, 1, 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, list, 2)
}
