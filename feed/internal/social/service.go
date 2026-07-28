package social

import (
	"context"
	"fmt"
	"log"
)

// BigVChecker 大V 判定器
type BigVChecker struct {
	repo      Repository
	threshold int
}

func NewBigVChecker(repo Repository, threshold int) *BigVChecker {
	return &BigVChecker{repo: repo, threshold: threshold}
}

func (c *BigVChecker) IsBigV(ctx context.Context, userID int64) bool {
	count := c.repo.FollowerCount(ctx, userID)
	return count >= c.threshold
}

// OutboxWriter 发件箱写入接口
type OutboxWriter interface {
	GetRange(ctx context.Context, userID int64, start, stop int64) ([]int64, error)
}

// TimelineCleaner 时间线清理接口（取关时移除帖子）
type TimelineCleaner interface {
	RemoveUserPosts(ctx context.Context, userID, targetUserID int64) error
}

// SyncWriter 收件箱写入接口（新粉同步历史帖子）
type SyncWriter interface {
	AddPost(ctx context.Context, userID int64, postID int64, score float64) error
}

// Service 社交服务
type Service struct {
	repo    Repository
	outbox  OutboxWriter
	bigV    *BigVChecker
	sync    SyncWriter
	cleaner TimelineCleaner
}

// NewService 创建社交服务
func NewService(repo Repository, outbox OutboxWriter, bigVThreshold int) *Service {
	return &Service{
		repo:   repo,
		outbox: outbox,
		bigV:   NewBigVChecker(repo, bigVThreshold),
	}
}

// SetSyncWriter 注入同步写入器（可选）
func (s *Service) SetSyncWriter(w SyncWriter) {
	s.sync = w
}

// SetCleaner 注入清理器（可选）
func (s *Service) SetCleaner(c TimelineCleaner) {
	s.cleaner = c
}

// Follow 关注用户
func (s *Service) Follow(ctx context.Context, followerID, followeeID int64) error {
	if followerID == followeeID {
		return fmt.Errorf("cannot follow self")
	}

	if err := s.repo.AddFollow(ctx, followerID, followeeID); err != nil {
		return err
	}

	// 关注后：非大V → 同步近30天历史帖子到新粉收件箱
	if !s.bigV.IsBigV(ctx, followeeID) && s.sync != nil && s.outbox != nil {
		go s.syncRecentPosts(followerID, followeeID)
	}

	// 检查被关注者是否升级为大V
	s.checkBigVUpgrade(ctx, followeeID)

	return nil
}

// Unfollow 取关用户
func (s *Service) Unfollow(ctx context.Context, followerID, followeeID int64) error {
	if followerID == followeeID {
		return fmt.Errorf("cannot unfollow self")
	}

	if err := s.repo.RemoveFollow(ctx, followerID, followeeID); err != nil {
		return err
	}

	// 取关后清理收件箱中该用户的帖子
	if s.cleaner != nil {
		if err := s.cleaner.RemoveUserPosts(ctx, followerID, followeeID); err != nil {
			log.Printf("clean timeline for user %d after unfollow %d: %v", followerID, followeeID, err)
		}
	}

	// 检查被取关者是否降级
	s.checkBigVDowngrade(ctx, followeeID)

	return nil
}

// syncRecentPosts 同步被关注者近期帖子到新粉收件箱
func (s *Service) syncRecentPosts(followerID, followeeID int64) {
	ctx := context.Background()
	ids, err := s.outbox.GetRange(ctx, followeeID, 0, -1)
	if err != nil {
		log.Printf("sync recent posts: get outbox for %d: %v", followeeID, err)
		return
	}

	// 只取最近 30 天的帖子（score < 30天前的忽略）
	for _, id := range ids {
		// 简化：全部同步，由 Timeline 裁剪控制上限
		if err := s.sync.AddPost(ctx, followerID, id, float64(id)); err != nil {
			log.Printf("sync post %d to %d: %v", id, followerID, err)
		}
	}
	log.Printf("synced %d posts from %d to new follower %d", len(ids), followeeID, followerID)
}

// checkBigVUpgrade 检查用户是否升级为大V
func (s *Service) checkBigVUpgrade(ctx context.Context, userID int64) {
	count := s.repo.FollowerCount(ctx, userID)
	if count >= s.bigV.threshold && !s.repo.IsBigVUser(ctx, userID) {
		s.repo.SetBigV(ctx, userID, true)
		log.Printf("user %d upgraded to BigV (follower count: %d)", userID, count)
	}
}

// checkBigVDowngrade 检查用户是否降级为普通用户
func (s *Service) checkBigVDowngrade(ctx context.Context, userID int64) {
	count := s.repo.FollowerCount(ctx, userID)
	if count < s.bigV.threshold && s.repo.IsBigVUser(ctx, userID) {
		s.repo.SetBigV(ctx, userID, false)
		log.Printf("user %d downgraded from BigV (follower count: %d)", userID, count)
	}
}

// GetFollowers 获取粉丝列表
func (s *Service) GetFollowers(ctx context.Context, userID int64) ([]int64, error) {
	return s.repo.GetFollowers(ctx, userID)
}

// GetFollowing 获取关注列表
func (s *Service) GetFollowing(ctx context.Context, userID int64) ([]int64, error) {
	return s.repo.GetFollowing(ctx, userID)
}

// IsBigV 判定是否大V
func (s *Service) IsBigV(ctx context.Context, userID int64) bool {
	return s.bigV.IsBigV(ctx, userID)
}
