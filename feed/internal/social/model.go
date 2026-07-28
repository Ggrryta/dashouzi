package social

import "context"

// Repository 社交关系持久化接口
type Repository interface {
	AddFollow(ctx context.Context, followerID, followeeID int64) error
	RemoveFollow(ctx context.Context, followerID, followeeID int64) error
	IsFollowing(ctx context.Context, followerID, followeeID int64) bool
	FollowerCount(ctx context.Context, userID int64) int
	GetFollowers(ctx context.Context, userID int64) ([]int64, error)
	GetFollowing(ctx context.Context, userID int64) ([]int64, error)
	SetBigV(ctx context.Context, userID int64, isBigV bool) error
	IsBigVUser(ctx context.Context, userID int64) bool
}
