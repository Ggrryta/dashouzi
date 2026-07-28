package ranking

import (
	"math"
	"time"
)

const (
	// 时间衰减重力因子，越大老帖衰减越快
	gravity = 1.5
	// 点赞权重
	likeWeight = 2
	// 评论权重
	commentWeight = 1
)

// CalculateScore HackerNews 风格热度分值
// score = (likes*2 + comments + 1) / (hours_since_epoch + 2)^1.5
// postTime: Unix timestamp of the post
func CalculateScore(likes, comments int, postTime int64) float64 {
	now := time.Now().Unix()
	hoursSince := float64(now-postTime) / 3600.0
	if hoursSince < 0 {
		hoursSince = 0
	}

	numerator := float64(likes*likeWeight + comments*commentWeight + 1)
	denominator := math.Pow(hoursSince+2, gravity)

	return numerator / denominator
}
