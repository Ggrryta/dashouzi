package feed

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// Outbox 用户发件箱（Redis Sorted Set）
type Outbox struct {
	rdb    *redis.Client
	maxLen int64
}

// NewOutbox 创建发件箱，maxLen 为保留最大条数
func NewOutbox(rdb *redis.Client, maxLen int64) *Outbox {
	return &Outbox{rdb: rdb, maxLen: maxLen}
}

func (o *Outbox) key(userID int64) string {
	return fmt.Sprintf("outbox:%d", userID)
}

// Add 添加帖子到发件箱，并裁剪到 maxLen
func (o *Outbox) Add(ctx context.Context, userID, postID int64, score float64) error {
	key := o.key(userID)
	pipe := o.rdb.Pipeline()

	// 添加
	pipe.ZAdd(ctx, key, redis.Z{Score: score, Member: postID})
	// 裁剪：保留 score 最大的 maxLen 条，即最近 maxLen 条
	// ZREMRANGEBYRANK key 0 -(maxLen+1) 移除排名最靠前的（score最小的）
	pipe.ZRemRangeByRank(ctx, key, 0, -(o.maxLen + 1))

	_, err := pipe.Exec(ctx)
	return err
}

// GetRange 按排名获取帖子 ID，start/stop 为索引（支持 0, -1 获取全部）
func (o *Outbox) GetRange(ctx context.Context, userID int64, start, stop int64) ([]int64, error) {
	key := o.key(userID)
	results, err := o.rdb.ZRange(ctx, key, start, stop).Result()
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

// Count 获取发件箱帖子数
func (o *Outbox) Count(ctx context.Context, userID int64) (int64, error) {
	return o.rdb.ZCard(ctx, o.key(userID)).Result()
}
