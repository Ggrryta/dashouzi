package feed

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// fakeRepo 内存假仓库，用于单元测试
type fakeRepo struct {
	posts  map[int64]*Post
	nextID int64
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{posts: make(map[int64]*Post)}
}

func (r *fakeRepo) Create(ctx context.Context, p *Post) error {
	if r.nextID == 0 {
		r.nextID = 1
	}
	p.ID = r.nextID
	r.nextID++
	r.posts[p.ID] = p
	return nil
}

func (r *fakeRepo) GetByID(ctx context.Context, id int64) (*Post, error) {
	p, ok := r.posts[id]
	if !ok {
		return nil, nil
	}
	return p, nil
}

func (r *fakeRepo) GetByUserID(ctx context.Context, userID int64, cursor int64, limit int) ([]*Post, error) {
	var result []*Post
	for _, p := range r.posts {
		if p.UserID == userID && p.ID > cursor {
			result = append(result, p)
		}
	}
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

// T1.5: 空内容拒绝
func TestCreatePost_EmptyContent(t *testing.T) {
	svc := NewService(newFakeRepo(), nil) // nil outbox for unit test
	_, err := svc.CreatePost(context.Background(), 1, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "content")
}

// T1.5b: 有效帖子创建
func TestCreatePost_Valid(t *testing.T) {
	svc := NewService(newFakeRepo(), nil)
	post, err := svc.CreatePost(context.Background(), 1, "Hello World")
	assert.NoError(t, err)
	assert.NotZero(t, post.ID)
	assert.Equal(t, int64(1), post.UserID)
	assert.Equal(t, "Hello World", post.Content)
	assert.False(t, post.CreatedAt.IsZero())
}
