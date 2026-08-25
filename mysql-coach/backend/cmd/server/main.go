package main

import (
	"log"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/mysql-coach/backend/internal/config"
	"github.com/mysql-coach/backend/internal/handler"
	"github.com/mysql-coach/backend/internal/llm"
	"github.com/mysql-coach/backend/internal/repository"
	"github.com/mysql-coach/backend/internal/service"
)

func main() {
	cfg := config.Load()

	// ===== 初始化数据库 =====
	db, err := repository.New(cfg.DB)
	if err != nil {
		log.Printf("⚠️ 数据库连接失败（功能受限模式）: %v", err)
	} else {
		log.Println("✅ 数据库连接成功")
	}

	// ===== 初始化 LLM =====
	llmProvider := llm.New(
		cfg.LLM.Provider, cfg.LLM.APIKey, cfg.LLM.BaseURL,
		cfg.LLM.Model, cfg.LLM.EmbeddingModel,
	)
	if cfg.LLM.APIKey != "" {
		log.Println("✅ LLM Provider 就绪:", cfg.LLM.Provider)
	} else {
		log.Println("⚠️ LLM_API_KEY 未配置，LLM 功能不可用")
	}

	// ===== 初始化 Repos =====
	scenarioRepo := repository.NewScenarioRepo(db)
	checkpointRepo := repository.NewCheckpointRepo(db)
	sessionRepo := repository.NewSessionRepo(db)
	progressRepo := repository.NewProgressRepo(db)
	noteRepo := repository.NewNoteRepo(db)
	domainRepo := repository.NewDomainRepo(db)

	// ===== 初始化 Services =====
	coachService := service.NewCoachService(
		scenarioRepo, sessionRepo, progressRepo, noteRepo, llmProvider,
	)

	// ===== 初始化 Handlers =====
	coachHandler := handler.NewCoachHandler(coachService)
	knowledgeHandler := handler.NewKnowledgeHandler(
		domainRepo, checkpointRepo, scenarioRepo, progressRepo,
	)

	// ===== 路由 =====
	gin.SetMode(cfg.Server.GinMode)
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{cfg.Server.FrontendURL, "http://localhost:3000"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "mysql-coach"})
	})

	api := r.Group("/api")
	{
		// 训练引擎
		coachHandler.RegisterRoutes(api)
		// 知识库查询
		knowledgeHandler.RegisterRoutes(api)
	}

	log.Printf("🚀 MySQL Coach 后端启动，端口 %s", cfg.Server.Port)
	if err := r.Run(":" + cfg.Server.Port); err != nil {
		log.Fatal(err)
	}
}
