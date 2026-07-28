package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"bloger/internal/dto"
	"bloger/internal/service"
	"bloger/pkg/errcode"
	"bloger/pkg/response"
)

type SearchHandler struct {
	svc *service.SearchService
}

func NewSearchHandler(svc *service.SearchService) *SearchHandler {
	return &SearchHandler{svc: svc}
}

func (h *SearchHandler) Search(c *gin.Context) {
	keyword := c.Query("q")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))

	articles, total, err := h.svc.Search(c.Request.Context(), service.SearchParams{
		Keyword: keyword,
		Page:    page,
		Size:    size,
	})
	if err != nil {
		response.Error(c, errcode.ErrBadRequest)
		return
	}

	var result []dto.ArticleResponse
	for _, a := range articles {
		result = append(result, toArticleResponse(a))
	}

	response.Success(c, gin.H{
		"articles": result,
		"total":    total,
		"page":     page,
		"size":     size,
	})
}
