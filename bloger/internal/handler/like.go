package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"bloger/internal/service"
	"bloger/pkg/errcode"
	"bloger/pkg/response"
)

type LikeHandler struct {
	svc *service.LikeService
}

func NewLikeHandler(svc *service.LikeService) *LikeHandler {
	return &LikeHandler{svc: svc}
}

type toggleLikeReq struct {
	TargetType string `json:"target_type" binding:"required,oneof=article comment"`
	TargetID   uint   `json:"target_id" binding:"required"`
}

func (h *LikeHandler) Toggle(c *gin.Context) {
	userID := c.GetUint("user_id")

	var req toggleLikeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrBadRequest)
		return
	}

	liked, err := h.svc.Toggle(c.Request.Context(), userID, req.TargetType, req.TargetID)
	if err != nil {
		if err == service.ErrTargetNotFound {
			response.Error(c, errcode.ErrTargetNotFound)
		} else {
			response.Error(c, errcode.ErrInternal)
		}
		return
	}

	response.Success(c, gin.H{"liked": liked})
}

func (h *LikeHandler) CheckStatus(c *gin.Context) {
	userID := c.GetUint("user_id")
	targetType := c.Query("target_type")
	targetID, _ := strconv.ParseUint(c.Query("target_id"), 10, 64)

	liked, err := h.svc.IsLiked(c.Request.Context(), userID, targetType, uint(targetID))
	if err != nil {
		response.Error(c, errcode.ErrInternal)
		return
	}

	response.Success(c, gin.H{"liked": liked})
}
