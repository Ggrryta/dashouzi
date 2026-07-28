package timeline

import (
	"context"

	"feed/internal/feed"
	"feed/internal/social"
)

// ---------- Inbox Adapter (feed.Timeline → InboxProvider) ----------

// InboxAdapter 将 feed.Timeline 适配为 InboxProvider
type InboxAdapter struct {
	tl *feed.Timeline
}

func NewInboxAdapter(tl *feed.Timeline) *InboxAdapter {
	return &InboxAdapter{tl: tl}
}

func (a *InboxAdapter) GetRecent(ctx context.Context, userID int64, limit int64, cursor float64) ([]int64, error) {
	return a.tl.GetRecent(ctx, userID, limit, cursor)
}

func (a *InboxAdapter) Count(ctx context.Context, userID int64) (int64, error) {
	return a.tl.Count(ctx, userID)
}

// ---------- Outbox Adapter (feed.Outbox → OutboxProvider) ----------

// OutboxAdapter 将 feed.Outbox 适配为 OutboxProvider
type OutboxAdapter struct {
	ob *feed.Outbox
}

func NewOutboxAdapter(ob *feed.Outbox) *OutboxAdapter {
	return &OutboxAdapter{ob: ob}
}

func (a *OutboxAdapter) GetRange(ctx context.Context, userID int64, start, stop int64) ([]int64, error) {
	return a.ob.GetRange(ctx, userID, start, stop)
}

func (a *OutboxAdapter) Count(ctx context.Context, userID int64) (int64, error) {
	return a.ob.Count(ctx, userID)
}

// ---------- Filler Adapter (feed.Repository → PostFiller) ----------

// FillerAdapter 将 feed.MysqlRepo 适配为 PostFiller
type FillerAdapter struct {
	repo feed.Repository
}

func NewFillerAdapter(repo feed.Repository) *FillerAdapter {
	return &FillerAdapter{repo: repo}
}

func (a *FillerAdapter) GetByID(ctx context.Context, id int64) (*postInfo, error) {
	p, err := a.repo.GetByID(ctx, id)
	if err != nil || p == nil {
		return nil, err
	}
	return &postInfo{
		ID:        p.ID,
		UserID:    p.UserID,
		Content:   p.Content,
		CreatedAt: p.CreatedAt,
	}, nil
}

// ---------- Social Adapter (social.Repository → SocialProvider) ----------

// SocialAdapter 将 social 仓库适配为 timeline 的 SocialProvider
type SocialAdapter struct {
	repo *social.MySQLRepo
}

func NewSocialAdapter(repo *social.MySQLRepo) *SocialAdapter {
	return &SocialAdapter{repo: repo}
}

func (a *SocialAdapter) GetFollowing(ctx context.Context, userID int64) ([]int64, error) {
	return a.repo.GetFollowing(ctx, userID)
}

func (a *SocialAdapter) IsBigV(ctx context.Context, userID int64) bool {
	return a.repo.IsBigVUser(ctx, userID)
}

// ---------- Timeline Cleaner (for social unfollow cleanup) ----------

// TimelineCleanerAdapter 取关时清理收件箱
type TimelineCleanerAdapter struct {
	tl     *feed.Timeline
	outbox *feed.Outbox
}

func NewTimelineCleanerAdapter(tl *feed.Timeline, outbox *feed.Outbox) *TimelineCleanerAdapter {
	return &TimelineCleanerAdapter{tl: tl, outbox: outbox}
}

// RemoveUserPosts 取关后移除对应用户的全部帖子
func (a *TimelineCleanerAdapter) RemoveUserPosts(ctx context.Context, userID, targetUserID int64) error {
	// 获取被取关用户发件箱的帖子
	postIDs, err := a.outbox.GetRange(ctx, targetUserID, 0, -1)
	if err != nil {
		return err
	}
	return a.tl.RemoveUserPosts(ctx, userID, postIDs)
}

// ---------- Sync Adapter (Timeline → social.SyncWriter) ----------

// SyncAdapter 将 feed.Timeline 适配为 social.SyncWriter
type SyncAdapter struct {
	tl *feed.Timeline
}

func NewSyncAdapter(tl *feed.Timeline) *SyncAdapter {
	return &SyncAdapter{tl: tl}
}

func (a *SyncAdapter) AddPost(ctx context.Context, userID, postID int64, score float64) error {
	return a.tl.AddPost(ctx, userID, postID, score)
}

// ---------- 编译期类型断言 ----------

var _ InboxProvider = (*InboxAdapter)(nil)
var _ OutboxProvider = (*OutboxAdapter)(nil)
var _ PostFiller = (*FillerAdapter)(nil)
var _ SocialProvider = (*SocialAdapter)(nil)
