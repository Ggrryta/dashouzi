package feed

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// RetryWorker 带重试和死信队列的异步 Worker
type RetryWorker struct {
	rdb       *redis.Client
	queueKey  string
	dlqKey    string
	maxRetry  int
}

// NewRetryWorker 创建 RetryWorker
func NewRetryWorker(rdb *redis.Client, queueKey, dlqKey string, maxRetry int) *RetryWorker {
	return &RetryWorker{
		rdb:      rdb,
		queueKey: queueKey,
		dlqKey:   dlqKey,
		maxRetry: maxRetry,
	}
}

// Enqueue 将任务推入主队列（Redis List 左侧推入）
func (w *RetryWorker) Enqueue(ctx context.Context, task DiffusionTask) error {
	data, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("marshal task: %w", err)
	}
	return w.rdb.LPush(ctx, w.queueKey, string(data)).Err()
}

// ProcessOne 从主队列取出一个任务并处理。失败则重试，达到上限入死信队列。
// handler 返回 nil 表示成功，非 nil 表示失败需重试。
func (w *RetryWorker) ProcessOne(ctx context.Context, handler func(DiffusionTask) error) {
	// 从右侧弹出（FIFO）
	data, err := w.rdb.RPop(ctx, w.queueKey).Result()
	if err == redis.Nil {
		return // 队列为空
	}
	if err != nil {
		log.Printf("retry worker: pop queue: %v", err)
		return
	}

	var task DiffusionTask
	if err := json.Unmarshal([]byte(data), &task); err != nil {
		log.Printf("retry worker: unmarshal task: %v", err)
		return
	}

	// 尝试处理（含重试），重试 inline 不重复入队
	for attempt := 0; attempt <= w.maxRetry; attempt++ {
		err := handler(task)
		if err == nil {
			return // 成功
		}

		if attempt < w.maxRetry {
			log.Printf("retry worker: task %d attempt %d failed: %v, retrying...",
				task.PostID, attempt+1, err)
			// 退避等待后重试
			time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
			continue
		}

		// 达到最大重试次数，入死信队列
		log.Printf("retry worker: task %d failed after %d retries, moving to DLQ",
			task.PostID, w.maxRetry)
		if err := w.rdb.LPush(ctx, w.dlqKey, data).Err(); err != nil {
			log.Printf("retry worker: push to DLQ failed: %v", err)
		}
	}
}

// Start 持续轮询处理任务
func (w *RetryWorker) Start(ctx context.Context, handler func(DiffusionTask) error) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				w.ProcessOne(ctx, handler)
				time.Sleep(100 * time.Millisecond) // 避免空轮询
			}
		}
	}()
}
