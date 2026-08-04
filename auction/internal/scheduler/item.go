package scheduler

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"auction/internal/service"
	"auction/pkg/logger"
)

// ItemScheduler 商品状态调度器
type ItemScheduler struct {
	itemService service.ItemService
	interval    time.Duration
	stopCh      chan struct{}
	stopOnce    sync.Once
	wg          sync.WaitGroup
	log         *zap.Logger
}

// NewItemScheduler 创建商品调度器
func NewItemScheduler(itemService service.ItemService, interval time.Duration) *ItemScheduler {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return &ItemScheduler{
		itemService: itemService,
		interval:    interval,
		stopCh:      make(chan struct{}),
		log:         logger.Log,
	}
}

// Start 启动调度器
func (s *ItemScheduler) Start() {
	s.wg.Add(1)
	go s.loop()
}

// Stop 停止调度器
func (s *ItemScheduler) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
}

func (s *ItemScheduler) loop() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// 立即执行一次
	s.tick()

	for {
		select {
		case <-ticker.C:
			s.tick()
		case <-s.stopCh:
			return
		}
	}
}

func (s *ItemScheduler) tick() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	now := time.Now()
	if err := s.itemService.StartPendingItems(ctx, now); err != nil {
		s.log.Error("start pending items failed", zap.Error(err))
	}
}
