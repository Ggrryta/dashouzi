package timeline

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器
type Handler struct {
	svc *Service
}

// NewHandler 创建 Handler
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// GetTimeline GET /api/v1/timeline?user_id=1&limit=20&cursor=123456
func (h *Handler) GetTimeline(c *gin.Context) {
	userID := parseQueryInt(c, "user_id")
	if userID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": "user_id required"})
		return
	}

	limit := parseQueryInt(c, "limit")
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	cursor := c.Query("cursor")

	resp, err := h.svc.GetTimeline(c.Request.Context(), userID, int(limit), cursor)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": resp})
}

func parseQueryInt(c *gin.Context, key string) int64 {
	v := c.Query(key)
	if v == "" {
		return 0
	}
	var n int64
	fmt.Sscanf(v, "%d", &n)
	return n
}


