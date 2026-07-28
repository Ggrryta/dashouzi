package feed

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

type createPostReq struct {
	UserID  int64  `json:"user_id" binding:"required"`
	Content string `json:"content" binding:"required"`
}

type createPostResp struct {
	PostID    int64  `json:"post_id"`
	CreatedAt string `json:"created_at"`
}

// CreatePost POST /api/v1/posts
func (h *Handler) CreatePost(c *gin.Context) {
	var req createPostReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": err.Error()})
		return
	}

	post, err := h.svc.CreatePost(c.Request.Context(), req.UserID, req.Content)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": createPostResp{
			PostID:    post.ID,
			CreatedAt: post.CreatedAt.Format("2006-01-02T15:04:05Z"),
		},
	})
}

// GetPost GET /api/v1/posts/:id
func (h *Handler) GetPost(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": "invalid id"})
		return
	}

	post, err := h.svc.GetPost(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "msg": err.Error()})
		return
	}
	if post == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "msg": "post not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": post})
}
