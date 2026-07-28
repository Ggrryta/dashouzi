package feed

import (
	"context"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// T4.8: 异步扩散重试 3 次后入死信队列
func TestRetryWorker_DeadLetter(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	worker := NewRetryWorker(rdb, "queue:diffusion", "dead_letter:diffusion", 3)
	ctx := context.Background()

	// 构造一个必定失败的任务
	task := DiffusionTask{PostID: 1, AuthorID: 100, Timestamp: 1000.0}

	// 推入主队列
	err := worker.Enqueue(ctx, task)
	assert.NoError(t, err)

	// 处理任务（会失败，触发重试）
	failCount := 0
	handler := func(task DiffusionTask) error {
		failCount++
		return errors.New("always fail")
	}

	worker.ProcessOne(ctx, handler)

	// 重试 3 次后应入死信队列
	dlqLen := rdb.LLen(ctx, "dead_letter:diffusion").Val()
	assert.Equal(t, int64(1), dlqLen, "after 3 retries, task should be in dead letter queue")

	// 主队列应为空
	qLen := rdb.LLen(ctx, "queue:diffusion").Val()
	assert.Equal(t, int64(0), qLen, "main queue should be empty")

	// 死信队列内容正确
	dlqData, _ := rdb.LPop(ctx, "dead_letter:diffusion").Result()
	assert.Contains(t, dlqData, `"post_id":1`)
	assert.Contains(t, dlqData, `"author_id":100`)

	_ = failCount
}

// 成功任务不触发重试
func TestRetryWorker_Success(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	worker := NewRetryWorker(rdb, "queue:diffusion", "dead_letter:diffusion", 3)
	ctx := context.Background()

	task := DiffusionTask{PostID: 2, AuthorID: 200, Timestamp: 2000.0}
	worker.Enqueue(ctx, task)

	handler := func(task DiffusionTask) error {
		return nil // 成功
	}

	worker.ProcessOne(ctx, handler)

	// 主队列和死信队列都应为空
	qLen := rdb.LLen(ctx, "queue:diffusion").Val()
	assert.Equal(t, int64(0), qLen)

	dlqLen := rdb.LLen(ctx, "dead_letter:diffusion").Val()
	assert.Equal(t, int64(0), dlqLen)
}
