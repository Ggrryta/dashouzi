package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mysql-coach/backend/internal/service"
)

// CoachHandler 训练相关 HTTP 接口
type CoachHandler struct {
	coach *service.CoachService
}

func NewCoachHandler(coach *service.CoachService) *CoachHandler {
	return &CoachHandler{coach: coach}
}

// RegisterRoutes 注册路由
func (h *CoachHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.POST("/coach/start", h.StartSession)
	r.POST("/coach/answer", h.SubmitAnswer)
}

// StartSession POST /api/coach/start
// Body: { "user_id": "xxx", "scenario_id": "mysql-scn-01" }
func (h *CoachHandler) StartSession(c *gin.Context) {
	var req struct {
		UserID     string `json:"user_id" binding:"required"`
		ScenarioID string `json:"scenario_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.coach.StartSession(c, req.UserID, req.ScenarioID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// SubmitAnswer POST /api/coach/answer
// Body: { "session_id":"xxx", "user_id":"xxx", "scenario_id":"xxx", "answer":"type是all", "current_level":1, "hints_used":0 }
func (h *CoachHandler) SubmitAnswer(c *gin.Context) {
	var req service.SubmitAnswerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.coach.SubmitAnswer(c, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}
