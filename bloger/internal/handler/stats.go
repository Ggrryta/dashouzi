package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"bloger/internal/service"
	"bloger/pkg/errcode"
	"bloger/pkg/response"
)

type StatsHandler struct {
	svc *service.StatsService
}

func NewStatsHandler(svc *service.StatsService) *StatsHandler {
	return &StatsHandler{svc: svc}
}

func (h *StatsHandler) Trending(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	articles, err := h.svc.Trending(c.Request.Context(), limit)
	if err != nil {
		response.Error(c, errcode.ErrInternal)
		return
	}

	var result []gin.H
	for _, a := range articles {
		result = append(result, gin.H{
			"id":         a.ID,
			"title":      a.Title,
			"slug":       a.Slug,
			"view_count": a.ViewCount,
		})
	}

	response.Success(c, gin.H{"articles": result})
}

func (h *StatsHandler) UserRanking(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	ranks, err := h.svc.UserRanking(c.Request.Context(), limit)
	if err != nil {
		response.Error(c, errcode.ErrInternal)
		return
	}

	response.Success(c, gin.H{"ranking": ranks})
}
