package ranking

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// HotFeed 热门 Feed ZSet
type HotFeed struct {
	rdb *redis.Client
	key string
}

// NewHotFeed 创建热门 Feed
func NewHotFeed(rdb *redis.Client) *HotFeed {
	return &HotFeed{rdb: rdb, key: "feed:hot"}
}

// UpdateScore 更新帖子热度分值
func (h *HotFeed) UpdateScore(ctx context.Context, postID int64, likes, comments int, postTime int64) {
	score := CalculateScore(likes, comments, postTime)
	h.rdb.ZAdd(ctx, h.key, redis.Z{Score: score, Member: postID})
}

// GetTop 获取 Top N
func (h *HotFeed) GetTop(ctx context.Context, limit int64) ([]int64, error) {
	results, err := h.rdb.ZRevRange(ctx, h.key, 0, limit-1).Result()
	if err != nil {
		return nil, err
	}
	ids := make([]int64, len(results))
	for i, v := range results {
		fmt.Sscanf(v, "%d", &ids[i])
	}
	return ids, nil
}

// Handler HTTP 处理器
type Handler struct {
	hot *HotFeed
}

// NewHandler 创建 Handler
func NewHandler(hot *HotFeed) *Handler {
	return &Handler{hot: hot}
}

// GetHotFeed GET /api/v1/feed/hot?limit=20
func (h *Handler) GetHotFeed(c *gin.Context) {
	limit := int64(20)
	if l := c.Query("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}
	if limit > 100 {
		limit = 100
	}

	ids, err := h.hot.GetTop(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": ids})
}
