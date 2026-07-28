package feed

import (
	"context"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
)

// Timeline 用户收件箱（Redis Sorted Set），存储推送到粉丝时间线的帖子
type Timeline struct {
	rdb    *redis.Client
	maxLen int64
}

// NewTimeline 创建时间线，maxLen 为保留最大条数
func NewTimeline(rdb *redis.Client, maxLen int64) *Timeline {
	return &Timeline{rdb: rdb, maxLen: maxLen}
}

func (t *Timeline) key(userID int64) string {
	return fmt.Sprintf("timeline:%d", userID)
}

// AddPost 添加帖子到收件箱，score 为时间戳（越大越新），超出 maxLen 自动裁剪
func (t *Timeline) AddPost(ctx context.Context, userID int64, postID int64, score float64) error {
	key := t.key(userID)
	pipe := t.rdb.Pipeline()
	pipe.ZAdd(ctx, key, redis.Z{Score: score, Member: postID})
	// 保留 score 最大的 maxLen 条，即最近 maxLen 条
	pipe.ZRemRangeByRank(ctx, key, 0, -(t.maxLen + 1))
	_, err := pipe.Exec(ctx)
	return err
}

// GetRecent 获取最近的 N 条帖子（按 score 降序），cursor 为时间戳游标（0 表示从最新开始）
func (t *Timeline) GetRecent(ctx context.Context, userID int64, limit int64, cursor float64) ([]int64, error) {
	var results []string
	var err error
	if cursor > 0 {
		results, err = t.rdb.ZRevRangeByScore(ctx, t.key(userID), &redis.ZRangeBy{
			Min:    "0",
			Max:    strconv.FormatFloat(cursor-1, 'f', 0, 64),
			Offset: 0,
			Count:  limit,
		}).Result()
	} else {
		results, err = t.rdb.ZRevRange(ctx, t.key(userID), 0, limit-1).Result()
	}
	if err != nil {
		return nil, err
	}
	ids := make([]int64, len(results))
	for i, v := range results {
		var id int64
		fmt.Sscanf(v, "%d", &id)
		ids[i] = id
	}
	return ids, nil
}

// Count 获取收件箱帖子数
func (t *Timeline) Count(ctx context.Context, userID int64) (int64, error) {
	return t.rdb.ZCard(ctx, t.key(userID)).Result()
}

// RemovePost 从收件箱移除指定帖子
func (t *Timeline) RemovePost(ctx context.Context, userID int64, postID int64) error {
	return t.rdb.ZRem(ctx, t.key(userID), postID).Err()
}

// RemoveUserPosts 移除收件箱中指定用户的所有帖子
// postIDs 是要移除的帖子 ID 列表
func (t *Timeline) RemoveUserPosts(ctx context.Context, userID int64, postIDs []int64) error {
	if len(postIDs) == 0 {
		return nil
	}
	members := make([]interface{}, len(postIDs))
	for i, id := range postIDs {
		members[i] = id
	}
	return t.rdb.ZRem(ctx, t.key(userID), members...).Err()
}
