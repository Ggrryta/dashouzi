//go:build integration
// +build integration

package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"bloger/internal/model"
)

func setupDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "host=postgres port=5432 user=bloger password=bloger123 dbname=bloger sslmode=disable TimeZone=Asia/Shanghai"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect db failed: %v", err)
	}
	// 自动建表
	db.AutoMigrate(&model.User{}, &model.Article{}, &model.Tag{}, &model.Comment{}, &model.Like{}, &model.Favorite{})
	return db
}

func cleanDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	db.Exec("DELETE FROM favorites")
	db.Exec("DELETE FROM likes")
	db.Exec("DELETE FROM comments")
	db.Exec("DELETE FROM article_tags")
	db.Exec("DELETE FROM tags")
	db.Exec("DELETE FROM articles")
	db.Exec("DELETE FROM users")
}

// ====== User Repo ======

func TestUserRepo_CreateAndFind(t *testing.T) {
	db := setupDB(t)
	defer cleanDB(t, db)
	repo := NewUserRepo(db)

	user := &model.User{Username: "itest", Email: "itest@test.com", PasswordHash: "hash", Role: "reader"}
	err := repo.Create(context.Background(), user)
	assert.NoError(t, err)
	assert.NotZero(t, user.ID)

	found, err := repo.FindByEmail(context.Background(), "itest@test.com")
	assert.NoError(t, err)
	assert.Equal(t, "itest", found.Username)

	foundByID, err := repo.FindByID(context.Background(), user.ID)
	assert.NoError(t, err)
	assert.Equal(t, "itest", foundByID.Username)
}

func TestUserRepo_FindByEmail_NotFound(t *testing.T) {
	db := setupDB(t)
	defer cleanDB(t, db)
	repo := NewUserRepo(db)

	found, err := repo.FindByEmail(context.Background(), "nobody@test.com")
	assert.NoError(t, err)
	assert.Nil(t, found)
}

// ====== Article Repo ======

func TestArticleRepo_CreateAndFind(t *testing.T) {
	db := setupDB(t)
	defer cleanDB(t, db)

	userRepo := NewUserRepo(db)
	user := &model.User{Username: "author", Email: "a@test.com", PasswordHash: "hash", Role: "author"}
	userRepo.Create(context.Background(), user)

	tagRepo := NewTagRepo(db)
	tag, _ := tagRepo.FindOrCreate(context.Background(), "Go")

	articleRepo := NewArticleRepo(db)
	article := &model.Article{
		AuthorID: user.ID,
		Title:    "Integration Test",
		Slug:     "integration-test",
		Content:  "Test content",
		Status:   "draft",
		Tags:     []model.Tag{*tag},
	}
	err := articleRepo.Create(context.Background(), article)
	assert.NoError(t, err)
	assert.NotZero(t, article.ID)

	found, err := articleRepo.FindByID(context.Background(), article.ID)
	assert.NoError(t, err)
	assert.Equal(t, "Integration Test", found.Title)
	assert.Len(t, found.Tags, 1)
	assert.Equal(t, "Go", found.Tags[0].Name)
}

func TestArticleRepo_List(t *testing.T) {
	db := setupDB(t)
	defer cleanDB(t, db)

	userRepo := NewUserRepo(db)
	user := &model.User{Username: "listtest", Email: "list@test.com", PasswordHash: "hash", Role: "author"}
	userRepo.Create(context.Background(), user)

	articleRepo := NewArticleRepo(db)
	for i := 0; i < 3; i++ {
		articleRepo.Create(context.Background(), &model.Article{
			AuthorID: user.ID,
			Title:    "Article " + string(rune('A'+i)),
			Slug:     "article-" + string(rune('a'+i)),
			Content:  "Content",
			Status:   "published",
		})
	}

	articles, total, err := articleRepo.List(context.Background(), ArticleListParams{
		Status: "published", Page: 1, Size: 10,
	})
	assert.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, articles, 3)
}

func TestArticleRepo_IncrementViewCount(t *testing.T) {
	db := setupDB(t)
	defer cleanDB(t, db)

	userRepo := NewUserRepo(db)
	user := &model.User{Username: "viewtest", Email: "view@test.com", PasswordHash: "hash", Role: "author"}
	userRepo.Create(context.Background(), user)

	articleRepo := NewArticleRepo(db)
	a := &model.Article{
		AuthorID: user.ID, Title: "Views", Slug: "views", Content: "x", Status: "published",
	}
	articleRepo.Create(context.Background(), a)

	err := articleRepo.IncrementViewCount(context.Background(), a.ID)
	assert.NoError(t, err)

	found, _ := articleRepo.FindByID(context.Background(), a.ID)
	assert.Equal(t, int64(1), found.ViewCount)
}

// ====== Comment Repo ======

func TestCommentRepo_CreateAndFind(t *testing.T) {
	db := setupDB(t)
	defer cleanDB(t, db)

	userRepo := NewUserRepo(db)
	user := &model.User{Username: "cmt", Email: "cmt@test.com", PasswordHash: "hash", Role: "reader"}
	userRepo.Create(context.Background(), user)

	articleRepo := NewArticleRepo(db)
	a := &model.Article{AuthorID: user.ID, Title: "T", Slug: "cmt-slug", Content: "x", Status: "published"}
	articleRepo.Create(context.Background(), a)

	commentRepo := NewCommentRepo(db)
	c := &model.Comment{ArticleID: a.ID, UserID: user.ID, Content: "Nice!"}
	err := commentRepo.Create(context.Background(), c)
	assert.NoError(t, err)

	comments, err := commentRepo.FindByArticle(context.Background(), a.ID)
	assert.NoError(t, err)
	assert.Len(t, comments, 1)
	assert.Equal(t, "Nice!", comments[0].Content)
}

func TestCommentRepo_SoftDelete(t *testing.T) {
	db := setupDB(t)
	defer cleanDB(t, db)

	userRepo := NewUserRepo(db)
	user := &model.User{Username: "delcmt", Email: "dc@test.com", PasswordHash: "hash", Role: "reader"}
	userRepo.Create(context.Background(), user)

	articleRepo := NewArticleRepo(db)
	a := &model.Article{AuthorID: user.ID, Title: "T", Slug: "del-slug", Content: "x", Status: "published"}
	articleRepo.Create(context.Background(), a)

	commentRepo := NewCommentRepo(db)
	c := &model.Comment{ArticleID: a.ID, UserID: user.ID, Content: "To delete"}
	commentRepo.Create(context.Background(), c)

	err := commentRepo.SoftDelete(context.Background(), c.ID)
	assert.NoError(t, err)

	found, _ := commentRepo.FindByID(context.Background(), c.ID)
	assert.NotNil(t, found)
	assert.True(t, found.IsDeleted)
}

// ====== Like Repo ======

func TestLikeRepo_CreateAndExists(t *testing.T) {
	db := setupDB(t)
	defer cleanDB(t, db)

	repo := NewLikeRepo(db)
	err := repo.Create(context.Background(), &model.Like{
		UserID: 1, TargetType: "article", TargetID: 1,
	})
	assert.NoError(t, err)

	exists, err := repo.Exists(context.Background(), 1, "article", 1)
	assert.NoError(t, err)
	assert.True(t, exists)

	err = repo.Delete(context.Background(), 1, "article", 1)
	assert.NoError(t, err)

	exists, _ = repo.Exists(context.Background(), 1, "article", 1)
	assert.False(t, exists)
}

// ====== Favorite Repo ======

func TestFavoriteRepo_CreateAndList(t *testing.T) {
	db := setupDB(t)
	defer cleanDB(t, db)

	repo := NewFavoriteRepo(db)
	repo.Create(context.Background(), &model.Favorite{UserID: 1, ArticleID: 1})
	repo.Create(context.Background(), &model.Favorite{UserID: 1, ArticleID: 2})

	favs, total, err := repo.List(context.Background(), 1, 1, 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, favs, 2)
}
