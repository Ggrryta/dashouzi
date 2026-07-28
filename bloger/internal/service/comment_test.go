package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"bloger/internal/model"
)

// ====== mock comment repo ======

type mockCommentRepo struct {
	comments map[uint]*model.Comment
	nextID   uint
}

func newMockCommentRepo() *mockCommentRepo {
	return &mockCommentRepo{comments: make(map[uint]*model.Comment), nextID: 1}
}

func (m *mockCommentRepo) Create(_ context.Context, c *model.Comment) error {
	c.ID = m.nextID
	m.nextID++
	m.comments[c.ID] = c
	return nil
}

func (m *mockCommentRepo) FindByArticle(_ context.Context, articleID uint) ([]*model.Comment, error) {
	var result []*model.Comment
	for _, c := range m.comments {
		if c.ArticleID == articleID && c.ParentID == nil && !c.IsDeleted {
			// 加载子回复
			for _, r := range m.comments {
				if r.ParentID != nil && *r.ParentID == c.ID && !r.IsDeleted {
					c.Replies = append(c.Replies, *r)
				}
			}
			result = append(result, c)
		}
	}
	return result, nil
}

func (m *mockCommentRepo) FindReplies(_ context.Context, parentID uint) ([]*model.Comment, error) {
	var result []*model.Comment
	for _, c := range m.comments {
		if c.ParentID != nil && *c.ParentID == parentID && !c.IsDeleted {
			result = append(result, c)
		}
	}
	return result, nil
}

func (m *mockCommentRepo) SoftDelete(_ context.Context, id uint) error {
	if c, ok := m.comments[id]; ok {
		c.IsDeleted = true
	}
	return nil
}

func (m *mockCommentRepo) FindByID(_ context.Context, id uint) (*model.Comment, error) {
	c, ok := m.comments[id]
	if !ok {
		return nil, nil
	}
	return c, nil
}

// ====== mock sensitive filter ======

type mockFilter struct {
	words []string
}

func (m *mockFilter) Match(text string) []string {
	var result []string
	for _, w := range m.words {
		if len(text) > len(w) {
			// 简单匹配
			for i := 0; i <= len(text)-len(w); i++ {
				if text[i:i+len(w)] == w {
					result = append(result, w)
					break
				}
			}
		}
	}
	return result
}

// ====== tests ======

func TestCommentCreate_Success(t *testing.T) {
	repo := newMockCommentRepo()
	filter := &mockFilter{}
	svc := NewCommentService(repo, filter)

	c, err := svc.Create(context.Background(), 1, 1, CreateCommentInput{
		Content: "好文章",
	})
	assert.NoError(t, err)
	assert.Equal(t, "好文章", c.Content)
	assert.Nil(t, c.ParentID)
	assert.False(t, c.IsDeleted)
}

func TestCommentCreate_WithParent(t *testing.T) {
	repo := newMockCommentRepo()
	filter := &mockFilter{}
	svc := NewCommentService(repo, filter)

	parent, _ := svc.Create(context.Background(), 1, 1, CreateCommentInput{Content: "parent"})
	reply, err := svc.Create(context.Background(), 2, 1, CreateCommentInput{
		Content:  "reply",
		ParentID: &parent.ID,
	})
	assert.NoError(t, err)
	assert.Equal(t, parent.ID, *reply.ParentID)
}

func TestCommentCreate_CrossArticleParent_Rejected(t *testing.T) {
	repo := newMockCommentRepo()
	svc := NewCommentService(repo, &mockFilter{})

	// parent 属于 article 1
	parent, _ := svc.Create(context.Background(), 1, 1, CreateCommentInput{Content: "parent"})
	// S4: 在 article 2 下回复 article 1 的 parent 应被拒
	_, err := svc.Create(context.Background(), 2, 2, CreateCommentInput{
		Content:  "cross",
		ParentID: &parent.ID,
	})
	assert.ErrorIs(t, err, ErrInvalidParent)
}

func TestCommentCreate_NonExistentParent_Rejected(t *testing.T) {
	repo := newMockCommentRepo()
	svc := NewCommentService(repo, &mockFilter{})

	fakeID := uint(999)
	// S4: parent 不存在应被拒
	_, err := svc.Create(context.Background(), 1, 1, CreateCommentInput{
		Content:  "reply",
		ParentID: &fakeID,
	})
	assert.ErrorIs(t, err, ErrInvalidParent)
}

func TestCommentCreate_SensitiveWord(t *testing.T) {
	repo := newMockCommentRepo()
	filter := &mockFilter{words: []string{"违禁词"}}
	svc := NewCommentService(repo, filter)

	_, err := svc.Create(context.Background(), 1, 1, CreateCommentInput{
		Content: "这是包含违禁词的内容",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sensitive")
}

func TestCommentCreate_EmptyContent(t *testing.T) {
	repo := newMockCommentRepo()
	svc := NewCommentService(repo, &mockFilter{})

	_, err := svc.Create(context.Background(), 1, 1, CreateCommentInput{
		Content: "",
	})
	assert.Error(t, err)
}

func TestCommentBuildTree_TwoLevels(t *testing.T) {
	repo := newMockCommentRepo()
	svc := NewCommentService(repo, &mockFilter{})

	// 创建顶级评论
	c1, _ := svc.Create(context.Background(), 1, 1, CreateCommentInput{Content: "top"})
	// 创建子回复
	svc.Create(context.Background(), 2, 1, CreateCommentInput{Content: "reply1", ParentID: &c1.ID})
	svc.Create(context.Background(), 2, 1, CreateCommentInput{Content: "reply2", ParentID: &c1.ID})

	comments, err := svc.GetByArticle(context.Background(), 1)
	assert.NoError(t, err)
	assert.Len(t, comments, 1)
	assert.Len(t, comments[0].Replies, 2)
}

func TestCommentDelete_Success(t *testing.T) {
	repo := newMockCommentRepo()
	svc := NewCommentService(repo, &mockFilter{})

	c, _ := svc.Create(context.Background(), 1, 1, CreateCommentInput{Content: "test"})

	err := svc.Delete(context.Background(), 1, "reader", c.ID)
	assert.NoError(t, err)

	deleted, _ := repo.FindByID(context.Background(), c.ID)
	assert.True(t, deleted.IsDeleted)
}

func TestCommentDelete_NotOwner(t *testing.T) {
	repo := newMockCommentRepo()
	svc := NewCommentService(repo, &mockFilter{})

	c, _ := svc.Create(context.Background(), 1, 1, CreateCommentInput{Content: "test"})

	err := svc.Delete(context.Background(), 2, "reader", c.ID) // 别人删
	assert.Error(t, err)
}

func TestCommentDelete_Admin(t *testing.T) {
	repo := newMockCommentRepo()
	svc := NewCommentService(repo, &mockFilter{})

	c, _ := svc.Create(context.Background(), 1, 1, CreateCommentInput{Content: "test"})

	// S6: admin 可删除他人评论
	err := svc.Delete(context.Background(), 2, "admin", c.ID)
	assert.NoError(t, err)
}
