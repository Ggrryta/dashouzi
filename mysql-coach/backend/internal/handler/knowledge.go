package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mysql-coach/backend/internal/repository"
)

// KnowledgeHandler 知识库查询接口（领域+考点+场景列表）
type KnowledgeHandler struct {
	domainRepo     *repository.DomainRepo
	checkpointRepo *repository.CheckpointRepo
	scenarioRepo   *repository.ScenarioRepo
	progressRepo   *repository.ProgressRepo
}

func NewKnowledgeHandler(
	dr *repository.DomainRepo,
	cr *repository.CheckpointRepo,
	sr *repository.ScenarioRepo,
	pr *repository.ProgressRepo,
) *KnowledgeHandler {
	return &KnowledgeHandler{
		domainRepo:     dr,
		checkpointRepo: cr,
		scenarioRepo:   sr,
		progressRepo:   pr,
	}
}

func (h *KnowledgeHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/domains", h.ListDomains)
	r.GET("/checkpoints/:domain", h.ListCheckpoints)
	r.GET("/scenarios/:domain", h.ListScenarios)
	r.GET("/progress/:userID/:domain", h.GetProgress)
}

// ListDomains GET /api/domains
func (h *KnowledgeHandler) ListDomains(c *gin.Context) {
	domains, err := h.domainRepo.List(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"domains": domains})
}

// ListCheckpoints GET /api/checkpoints/mysql
func (h *KnowledgeHandler) ListCheckpoints(c *gin.Context) {
	domainID := c.Param("domain")
	checkpoints, err := h.checkpointRepo.ListByDomain(c, domainID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"checkpoints": checkpoints, "total": len(checkpoints)})
}

// ListScenarios GET /api/scenarios/mysql
func (h *KnowledgeHandler) ListScenarios(c *gin.Context) {
	domainID := c.Param("domain")
	scenarios, err := h.scenarioRepo.ListByDomain(c, domainID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"scenarios": scenarios, "total": len(scenarios)})
}

// GetProgress GET /api/progress/:userID/mysql
func (h *KnowledgeHandler) GetProgress(c *gin.Context) {
	userID := c.Param("userID")
	domainID := c.Param("domain")
	progress, err := h.progressRepo.ListByUser(c, userID, domainID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"progress": progress})
}
