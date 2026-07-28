package router

import (
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"bloger/internal/handler"
	"bloger/internal/middleware"
	"bloger/internal/repository"
	"bloger/internal/service"
	"bloger/pkg/errcode"
	"bloger/pkg/jwt"
	"bloger/pkg/response"
	"bloger/pkg/sensitive"
)

func Setup(db *gorm.DB, jwtService *jwt.JWT, sensitiveWords []string) *gin.Engine {
	r := gin.New()

	r.Use(middleware.Recovery())
	r.Use(middleware.Logger())

	// 初始化依赖
	userRepo := repository.NewUserRepo(db)
	userSvc := service.NewUserService(userRepo)
	userHandler := handler.NewUserHandler(userSvc, jwtService)

	articleRepo := repository.NewArticleRepo(db)
	tagRepo := repository.NewTagRepo(db)
	articleSvc := service.NewArticleService(articleRepo, tagRepo)
	articleHandler := handler.NewArticleHandler(articleSvc)

	commentRepo := repository.NewCommentRepo(db)
	filter := sensitive.New()
	if len(sensitiveWords) > 0 {
		filter.AddWords(sensitiveWords...)
	}
	commentSvc := service.NewCommentService(commentRepo, filter)
	commentHandler := handler.NewCommentHandler(commentSvc)

	likeRepo := repository.NewLikeRepo(db)
	likeSvc := service.NewLikeService(likeRepo, articleRepo, commentRepo)
	likeHandler := handler.NewLikeHandler(likeSvc)

	favRepo := repository.NewFavoriteRepo(db)
	favSvc := service.NewFavoriteService(favRepo, articleRepo)
	favHandler := handler.NewFavoriteHandler(favSvc)

	searchRepo := repository.NewSearchRepo(db)
	searchSvc := service.NewSearchService(searchRepo)
	searchHandler := handler.NewSearchHandler(searchSvc)

	statsRepo := repository.NewStatsRepo(db)
	statsSvc := service.NewStatsService(statsRepo)
	statsHandler := handler.NewStatsHandler(statsSvc)

	r.GET("/api/v1/ping", func(c *gin.Context) {
		response.Success(c, "pong")
	})

	v1 := r.Group("/api/v1")
	{
		// 限流保护
		rateLimit := v1.Group("")
		rateLimit.Use(middleware.RateLimit(30, time.Minute))
		{
			rateLimit.POST("/users/login", userHandler.Login)
			rateLimit.POST("/users/register", userHandler.Register)
		}

		// 公开接口
		v1.GET("/articles", articleHandler.List)
		v1.GET("/articles/:id", articleHandler.Get)
		v1.GET("/articles/:id/comments", commentHandler.GetByArticle)
		v1.GET("/search", searchHandler.Search)
		v1.GET("/stats/trending", statsHandler.Trending)
		v1.GET("/stats/users", statsHandler.UserRanking)
		v1.GET("/tags", func(c *gin.Context) {
			tags, err := tagRepo.List(c.Request.Context())
			if err != nil {
				response.Error(c, errcode.ErrInternal)
				return
			}
			response.Success(c, tags)
		})

		// 需要认证的接口
		auth := v1.Group("")
		auth.Use(middleware.Auth(jwtService))
		{
			auth.GET("/users/me", userHandler.GetMe)

			// 文章:创建需 author/admin(S1);更新/删除/状态变更由 service 校验所有权+admin
			auth.POST("/articles", middleware.Role("author"), articleHandler.Create)
			auth.PUT("/articles/:id", articleHandler.Update)
			auth.DELETE("/articles/:id", articleHandler.Delete)
			auth.PATCH("/articles/:id/status", articleHandler.ChangeStatus)

			// 评论(限流)
			commentAuth := auth.Group("")
			commentAuth.Use(middleware.RateLimit(20, time.Minute))
			{
				commentAuth.POST("/comments", commentHandler.Create)
				commentAuth.DELETE("/comments/:id", commentHandler.Delete)
			}

			// 点赞
			auth.POST("/likes", likeHandler.Toggle)
			auth.GET("/likes/check", likeHandler.CheckStatus)

			// 收藏
			auth.POST("/favorites", favHandler.Toggle)
			auth.GET("/favorites", favHandler.List)
		}
	}

	return r
}
