package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

	"feed/internal/feed"
	"feed/internal/middleware"
	"feed/internal/ranking"
	"feed/internal/social"
	"feed/internal/timeline"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	// MySQL
	dsn := getEnv("MYSQL_DSN", "root:root@tcp(mysql:3306)/feed?parseTime=true")
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("mysql connect: %v", err)
	}
	defer db.Close()

	// Redis
	redisAddr := getEnv("REDIS_ADDR", "redis:6379")
	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})

	// Repository
	repo := feed.NewMySQLRepo(db)
	if err := repo.InitSchema(context.Background()); err != nil {
		log.Printf("WARN: init schema: %v", err)
	}

	// Outbox + Timeline + Diffusion
	outbox := feed.NewOutbox(rdb, 500)
	tl := feed.NewTimeline(rdb, 1000)

	// Social repo
	socialRepo := social.NewMySQLRepo(db)
	if err := socialRepo.InitSchema(context.Background()); err != nil {
		log.Printf("WARN: init social schema: %v", err)
	}

	// FollowerProvider adapter
	followAdapter := &socialAdapter{repo: socialRepo}
	diffusion := feed.NewDiffusion(followAdapter, tl)

	// Feed Service
	svc := feed.NewService(repo, outbox)
	svc.SetDiffusion(diffusion)

	// Timeline Service
	timelineSvc := timeline.NewService(
		timeline.NewInboxAdapter(tl),
		timeline.NewOutboxAdapter(outbox),
		timeline.NewFillerAdapter(repo),
		timeline.NewSocialAdapter(socialRepo),
	)

	// Social Service (with sync & cleaner)
	socialSvc := social.NewService(socialRepo, outbox, 100000)
	socialSvc.SetSyncWriter(timeline.NewSyncAdapter(tl))
	socialSvc.SetCleaner(timeline.NewTimelineCleanerAdapter(tl, outbox))

	// Hot Feed
	hotFeed := ranking.NewHotFeed(rdb)

	// Retry Worker
	retryWorker := feed.NewRetryWorker(rdb, "queue:diffusion", "dead_letter:diffusion", 3)

	// Handlers
	handler := feed.NewHandler(svc)
	socialHandler := social.NewHandler(socialSvc)
	timelineHandler := timeline.NewHandler(timelineSvc)
	rankingHandler := ranking.NewHandler(hotFeed)

	// Router
	r := gin.Default()

	// Rate limit middleware
	rateLimiter := middleware.NewRateLimiter(60, time.Minute)
	r.Use(rateLimiter.Middleware())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := r.Group("/api/v1")
	{
		api.POST("/posts", handler.CreatePost)
		api.GET("/posts/:id", handler.GetPost)
		api.POST("/follow", socialHandler.Follow)
		api.POST("/unfollow", socialHandler.Unfollow)
		api.GET("/following", socialHandler.GetFollowing)
		api.GET("/followers", socialHandler.GetFollowers)
		api.GET("/timeline", timelineHandler.GetTimeline)
		api.GET("/feed/hot", rankingHandler.GetHotFeed)
	}

	log.Printf("Feed server starting on :8080")

	// Start retry worker (already spawns goroutine internally)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	retryWorker.Start(ctx, func(task feed.DiffusionTask) error {
		_, err := diffusion.Spread(ctx, task.AuthorID, task.PostID, task.Timestamp)
		return err
	})

	r.Run(":8080")
}

// socialAdapter 将 social.MySQLRepo 适配为 feed.FollowerProvider
type socialAdapter struct {
	repo *social.MySQLRepo
}

func (a *socialAdapter) GetFollowers(ctx context.Context, userID int64) ([]int64, error) {
	return a.repo.GetFollowers(ctx, userID)
}

func (a *socialAdapter) IsBigV(ctx context.Context, userID int64) bool {
	return a.repo.IsBigVUser(ctx, userID)
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
