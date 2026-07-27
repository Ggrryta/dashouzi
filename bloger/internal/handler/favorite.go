package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"bloger/internal/service"
	"bloger/pkg/errcode"
	"bloger/pkg/response"
)

type FavoriteHandler struct {
	svc *service.FavoriteService
}

func NewFavoriteHandler(svc *service.FavoriteService) *FavoriteHandler {
	return &FavoriteHandler{svc: svc}
}

type toggleFavReq struct {
	ArticleID uint `json:"article_id" binding:"required"`
}

func (h *FavoriteHandler) Toggle(c *gin.Context) {
	userID := c.GetUint("user_id")

	var req toggleFavReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrBadRequest)
		return
	}

	faved, err := h.svc.Toggle(c.Request.Context(), userID, req.ArticleID)
	if err != nil {
		response.Error(c, errcode.ErrInternal)
		return
	}

	response.Success(c, gin.H{"favorited": faved})
}

func (h *FavoriteHandler) List(c *gin.Context) {
	userID := c.GetUint("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))

	favs, total, err := h.svc.List(c.Request.Context(), userID, page, size)
	if err != nil {
		response.Error(c, errcode.ErrInternal)
		return
	}

	var ids []uint
	for _, f := range favs {
		ids = append(ids, f.ArticleID)
	}

	response.Success(c, gin.H{
		"article_ids": ids,
		"total":       total,
		"page":        page,
	})
}
