package feed

import (
	"context"
	"log"
	"sync"
)

// FollowerProvider 粉丝列表提供者接口
type FollowerProvider interface {
	GetFollowers(ctx context.Context, userID int64) ([]int64, error)
	IsBigV(ctx context.Context, userID int64) bool
}

// Diffusion 写扩散——将普通用户的帖子推送到粉丝收件箱
type Diffusion struct {
	follows  FollowerProvider
	timeline *Timeline
}

// NewDiffusion 创建扩散器
func NewDiffusion(follows FollowerProvider, timeline *Timeline) *Diffusion {
	return &Diffusion{follows: follows, timeline: timeline}
}

// DiffusionTask 异步扩散任务
type DiffusionTask struct {
	PostID    int64   `json:"post_id"`
	AuthorID  int64   `json:"author_id"`
	Timestamp float64 `json:"timestamp"`
}

// Spread 同步扩散：将帖子推送到所有粉丝收件箱（仅供小粉丝量场景）
func (d *Diffusion) Spread(ctx context.Context, authorID, postID int64, timestamp float64) (int, error) {
	// 大V 不扩散
	if d.follows.IsBigV(ctx, authorID) {
		return 0, nil
	}

	followers, err := d.follows.GetFollowers(ctx, authorID)
	if err != nil {
		return 0, err
	}

	for _, fid := range followers {
		if err := d.timeline.AddPost(ctx, fid, postID, timestamp); err != nil {
			log.Printf("spread to follower %d failed: %v", fid, err)
		}
	}
	return len(followers), nil
}

// SpreadAsync 异步扩散到粉丝收件箱
func (d *Diffusion) SpreadAsync(authorID, postID int64, timestamp float64) {
	go func() {
		ctx := context.Background()
		n, err := d.Spread(ctx, authorID, postID, timestamp)
		if err != nil {
			log.Printf("async diffusion for post %d failed: %v", postID, err)
			return
		}
		if n > 0 {
			log.Printf("diffused post %d to %d followers", postID, n)
		}
	}()
}

// DiffusionWorker 异步扩散 Worker
type DiffusionWorker struct {
	diffusion *Diffusion
	queue     chan DiffusionTask
	wg        sync.WaitGroup
}

// NewDiffusionWorker 创建异步扩散 Worker
func NewDiffusionWorker(d *Diffusion, bufferSize int) *DiffusionWorker {
	return &DiffusionWorker{
		diffusion: d,
		queue:     make(chan DiffusionTask, bufferSize),
	}
}

// Enqueue 将扩散任务加入队列（非阻塞）
func (w *DiffusionWorker) Enqueue(task DiffusionTask) {
	select {
	case w.queue <- task:
	default:
		log.Printf("diffusion queue full, dropping task for post %d", task.PostID)
	}
}

// Start 启动 Worker 处理扩散任务
func (w *DiffusionWorker) Start(ctx context.Context) {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case task := <-w.queue:
				w.diffusion.Spread(ctx, task.AuthorID, task.PostID, task.Timestamp)
			}
		}
	}()
}

// Stop 停止 Worker（等待全部任务处理完）
func (w *DiffusionWorker) Stop() {
	close(w.queue)
	w.wg.Wait()
}
