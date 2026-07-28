package timeline

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ---------- fake providers ----------

type fakeInbox struct {
	data map[int64][]int64 // userID -> sorted postIDs (newest first in terms of semantics)
}

func (f *fakeInbox) GetRecent(ctx context.Context, userID int64, limit int64, cursor float64) ([]int64, error) {
	ids := f.data[userID]
	if len(ids) > int(limit) {
		return ids[:limit], nil
	}
	return ids, nil
}

func (f *fakeInbox) Count(ctx context.Context, userID int64) (int64, error) {
	return int64(len(f.data[userID])), nil
}

type fakeOutbox struct {
	data map[int64][]int64
}

func (f *fakeOutbox) GetRange(ctx context.Context, userID int64, start, stop int64) ([]int64, error) {
	return f.data[userID], nil
}

func (f *fakeOutbox) Count(ctx context.Context, userID int64) (int64, error) {
	return int64(len(f.data[userID])), nil
}

type fakeFiller struct {
	posts map[int64]*postInfo
}

func (f *fakeFiller) GetByID(ctx context.Context, id int64) (*postInfo, error) {
	return f.posts[id], nil
}

type fakeSocialTl struct {
	following map[int64][]int64
	bigVs     map[int64]bool
}

func (f *fakeSocialTl) GetFollowing(ctx context.Context, userID int64) ([]int64, error) {
	return f.following[userID], nil
}

func (f *fakeSocialTl) IsBigV(ctx context.Context, userID int64) bool {
	return f.bigVs[userID]
}

// ---------- tests ----------

// T3.1: 收件箱充足 → 直接返回
func TestTimelineService_InboxSufficient(t *testing.T) {
	ctx := context.Background()
	inbox, outbox, filler, social := newFakeProviders()

	// 用户1收件箱有10条帖子
	for i := int64(1); i <= 10; i++ {
		inbox.data[1] = append(inbox.data[1], i)
		filler.posts[i] = &postInfo{ID: i, UserID: i + 100, Content: "test", CreatedAt: time.Now()}
	}

	svc := NewService(inbox, outbox, filler, social)
	resp, err := svc.GetTimeline(ctx, 1, 5, "")
	assert.NoError(t, err)
	assert.Equal(t, 5, len(resp.Items))
	assert.True(t, resp.HasMore)
}

// 收件箱数据恰好够
func TestTimelineService_InboxExact(t *testing.T) {
	ctx := context.Background()
	inbox, outbox, filler, social := newFakeProviders()

	for i := int64(1); i <= 5; i++ {
		inbox.data[1] = append(inbox.data[1], i)
		filler.posts[i] = &postInfo{ID: i, UserID: 100, Content: "test", CreatedAt: time.Now()}
	}

	svc := NewService(inbox, outbox, filler, social)
	resp, err := svc.GetTimeline(ctx, 1, 5, "")
	assert.NoError(t, err)
	assert.Equal(t, 5, len(resp.Items))
	assert.False(t, resp.HasMore) // 恰好够 → 不设 has_more
}

// T3.2: 收件箱不足 + 关注大V → Pull 补充
func TestTimelineService_InboxInsufficient_PullBigV(t *testing.T) {
	ctx := context.Background()
	inbox, outbox, filler, social := newFakeProviders()

	// 收件箱只有 2 条
	inbox.data[1] = []int64{1, 2}
	filler.posts[1] = &postInfo{ID: 1, UserID: 100, Content: "inbox1", CreatedAt: time.Now()}
	filler.posts[2] = &postInfo{ID: 2, UserID: 200, Content: "inbox2", CreatedAt: time.Now().Add(-time.Hour)}

	// 关注了大V 100
	social.following[1] = []int64{100}
	social.bigVs[100] = true

	// 大V 发件箱有 3 条
	outbox.data[100] = []int64{10, 11, 12}
	filler.posts[10] = &postInfo{ID: 10, UserID: 100, Content: "bigv1", CreatedAt: time.Now().Add(-2 * time.Hour)}
	filler.posts[11] = &postInfo{ID: 11, UserID: 100, Content: "bigv2", CreatedAt: time.Now().Add(-3 * time.Hour)}
	filler.posts[12] = &postInfo{ID: 12, UserID: 100, Content: "bigv3", CreatedAt: time.Now().Add(-4 * time.Hour)}

	svc := NewService(inbox, outbox, filler, social)
	resp, err := svc.GetTimeline(ctx, 1, 5, "")
	assert.NoError(t, err)
	assert.Equal(t, 5, len(resp.Items)) // 2 inbox + 3 bigV outbox = 5
}

// T3.10: 无关注 → 时间线为空
func TestTimelineService_Empty(t *testing.T) {
	ctx := context.Background()
	inbox, outbox, filler, social := newFakeProviders()

	svc := NewService(inbox, outbox, filler, social)
	resp, err := svc.GetTimeline(ctx, 999, 20, "")
	assert.NoError(t, err)
	assert.Empty(t, resp.Items)
	assert.False(t, resp.HasMore)
}

// 关注普通用户（非大V）→ 不 Pull 发件箱，仅走 Push 收件箱
func TestTimelineService_NormalUser_NoPull(t *testing.T) {
	ctx := context.Background()
	inbox, outbox, filler, social := newFakeProviders()

	inbox.data[1] = []int64{1}
	filler.posts[1] = &postInfo{ID: 1, UserID: 200, Content: "normal", CreatedAt: time.Now()}

	social.following[1] = []int64{200}
	social.bigVs[200] = false // 不是大V

	// 发件箱有数据但还是不应该 pull
	outbox.data[200] = []int64{99}
	filler.posts[99] = &postInfo{ID: 99, UserID: 200, Content: "should not appear", CreatedAt: time.Now()}

	svc := NewService(inbox, outbox, filler, social)
	resp, err := svc.GetTimeline(ctx, 1, 20, "")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(resp.Items))
	assert.Equal(t, int64(1), resp.Items[0].PostID)
}

// 去重测试
func TestTimelineService_Dedup(t *testing.T) {
	ctx := context.Background()
	inbox, outbox, filler, social := newFakeProviders()

	// 收件箱和大V发件箱有重复的帖子
	inbox.data[1] = []int64{1, 2}
	outbox.data[100] = []int64{1, 3} // 1 重复
	filler.posts[1] = &postInfo{ID: 1, UserID: 100, Content: "dup", CreatedAt: time.Now()}
	filler.posts[2] = &postInfo{ID: 2, UserID: 200, Content: "a", CreatedAt: time.Now()}
	filler.posts[3] = &postInfo{ID: 3, UserID: 100, Content: "b", CreatedAt: time.Now().Add(-time.Hour)}

	social.following[1] = []int64{100}
	social.bigVs[100] = true

	svc := NewService(inbox, outbox, filler, social)
	resp, err := svc.GetTimeline(ctx, 1, 10, "")
	assert.NoError(t, err)
	assert.Equal(t, 3, len(resp.Items)) // 去重后只有 3 条
}

// helpers
func newFakeProviders() (*fakeInbox, *fakeOutbox, *fakeFiller, *fakeSocialTl) {
	return &fakeInbox{data: make(map[int64][]int64)},
		&fakeOutbox{data: make(map[int64][]int64)},
		&fakeFiller{posts: make(map[int64]*postInfo)},
		&fakeSocialTl{following: make(map[int64][]int64), bigVs: make(map[int64]bool)}
}
