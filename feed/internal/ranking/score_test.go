package ranking

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// T4.1: 新帖基础分
func TestHotScore_NewPost(t *testing.T) {
	now := time.Now().Unix()
	score := CalculateScore(0, 0, now)
	assert.Greater(t, score, 0.0, "new post should have positive base score")
}

// T4.2: 点赞加权
func TestHotScore_LikesWeight(t *testing.T) {
	now := time.Now().Unix()
	scoreNoLike := CalculateScore(0, 0, now)
	scoreWithLike := CalculateScore(10, 0, now)
	assert.Greater(t, scoreWithLike, scoreNoLike, "likes should increase score")
}

// T4.3: 时间衰减
func TestHotScore_TimeDecay(t *testing.T) {
	now := time.Now().Unix()
	scoreNew := CalculateScore(0, 0, now)
	scoreOld := CalculateScore(0, 0, now-86400*7) // 7天前
	assert.Greater(t, scoreNew, scoreOld, "newer post should rank higher than 7-day-old post")
}

// 评论加权
func TestHotScore_CommentsWeight(t *testing.T) {
	now := time.Now().Unix()
	scoreNoComment := CalculateScore(0, 0, now)
	scoreWithComment := CalculateScore(0, 5, now)
	assert.Greater(t, scoreWithComment, scoreNoComment, "comments should increase score")
}

// 同时点赞和评论叠加
func TestHotScore_LikesAndComments(t *testing.T) {
	now := time.Now().Unix()
	score1 := CalculateScore(5, 3, now)
	score2 := CalculateScore(8, 0, now)
	// 按 HackerNews 公式: (likes*2 + comments) / time_factor
	// score1: (10+3)=13, score2: (16+0)=16 → score2 > score1
	assert.Greater(t, score2, score1, "8 likes should outweigh 5 likes + 3 comments")
}

// 老帖即使高赞也低于新帖
func TestHotScore_OldHighLikes(t *testing.T) {
	now := time.Now().Unix()
	scoreOldPopular := CalculateScore(100, 50, now-86400*30) // 30天前，高赞
	scoreNewUnpopular := CalculateScore(1, 0, now)            // 刚刚，低赞
	// 时间衰减应足够强，新帖比 30 天前高赞帖分数高
	assert.Greater(t, scoreNewUnpopular, scoreOldPopular,
		"fresh post should beat 30-day-old popular post due to time decay")
}
