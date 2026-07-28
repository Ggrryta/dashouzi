package social

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

type followReq struct {
	FollowerID int64 `json:"follower_id" binding:"required"`
	FolloweeID int64 `json:"followee_id" binding:"required"`
}

// Follow POST /api/v1/follow
func (h *Handler) Follow(c *gin.Context) {
	var req followReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": err.Error()})
		return
	}
	if err := h.svc.Follow(c.Request.Context(), req.FollowerID, req.FolloweeID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": "ok"})
}

// Unfollow POST /api/v1/unfollow
func (h *Handler) Unfollow(c *gin.Context) {
	var req followReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": err.Error()})
		return
	}
	if err := h.svc.Unfollow(c.Request.Context(), req.FollowerID, req.FolloweeID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": "ok"})
}

// GetFollowers GET /api/v1/followers?user_id=1
func (h *Handler) GetFollowers(c *gin.Context) {
	userID := parseIntParam(c, "user_id")
	followers, err := h.svc.GetFollowers(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": followers})
}

// GetFollowing GET /api/v1/following?user_id=1
func (h *Handler) GetFollowing(c *gin.Context) {
	userID := parseIntParam(c, "user_id")
	following, err := h.svc.GetFollowing(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": following})
}

func parseIntParam(c *gin.Context, key string) int64 {
	var val int64
	if v, ok := c.GetQuery(key); ok {
		fmt.Sscanf(v, "%d", &val)
	}
	return val
}
