package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"bloger/internal/dto"
	"bloger/internal/model"
	"bloger/internal/service"
	"bloger/pkg/errcode"
	"bloger/pkg/response"
)

type ArticleHandler struct {
	svc *service.ArticleService
}

func NewArticleHandler(svc *service.ArticleService) *ArticleHandler {
	return &ArticleHandler{svc: svc}
}

type createArticleReq struct {
	Title    string   `json:"title" binding:"required,min=1,max=256"`
	Content  string   `json:"content" binding:"required"`
	Summary  string   `json:"summary"`
	CoverURL string   `json:"cover_url"`
	TagNames []string `json:"tags"`
}

type updateArticleReq struct {
	Title    string   `json:"title"`
	Content  string   `json:"content"`
	Summary  string   `json:"summary"`
	CoverURL string   `json:"cover_url"`
	TagNames []string `json:"tags"`
}

type changeStatusReq struct {
	Status string `json:"status" binding:"required"`
}

func (h *ArticleHandler) Create(c *gin.Context) {
	userID := c.GetUint("user_id")

	var req createArticleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrBadRequest)
		return
	}

	article, err := h.svc.Create(c.Request.Context(), userID, service.CreateArticleInput{
		Title:    req.Title,
		Content:  req.Content,
		Summary:  req.Summary,
		CoverURL: req.CoverURL,
		TagNames: req.TagNames,
	})
	if err != nil {
		response.Error(c, errcode.ErrBadRequest)
		return
	}

	// 回查获取完整的 author 和 tags
	article, _ = h.svc.GetByID(c.Request.Context(), article.ID)

	response.Success(c, toArticleResponse(article))
}

func (h *ArticleHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrBadRequest)
		return
	}

	// S2: 公开接口仅返回 published 文章,并同步累加浏览量
	article, err := h.svc.GetPublicByID(c.Request.Context(), uint(id))
	if err != nil {
		response.Error(c, errcode.ErrArticleNotFound)
		return
	}

	response.Success(c, toArticleResponse(article))
}

func (h *ArticleHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))
	status := c.DefaultQuery("status", "published")
	tagName := c.Query("tag")

	articles, total, err := h.svc.List(c.Request.Context(), service.ListArticleParams{
		Status:  status,
		TagName: tagName,
		Page:    page,
		Size:    size,
	})
	if err != nil {
		response.Error(c, errcode.ErrInternal)
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

func (h *ArticleHandler) Update(c *gin.Context) {
	userID := c.GetUint("user_id")
	role := c.GetString("role")
	articleID := getArticleID(c)

	var req updateArticleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrBadRequest)
		return
	}

	err := h.svc.Update(c.Request.Context(), userID, role, articleID, service.UpdateArticleInput{
		Title:    req.Title,
		Content:  req.Content,
		Summary:  req.Summary,
		CoverURL: req.CoverURL,
		TagNames: req.TagNames,
	})
	if err != nil {
		switch err {
		case service.ErrArticleNotFound:
			response.Error(c, errcode.ErrArticleNotFound)
		case service.ErrNotOwner:
			response.Error(c, errcode.ErrNotArticleOwner)
		default:
			response.Error(c, errcode.ErrBadRequest)
		}
		return
	}

	// 查询更新后的文章返回
	article, _ := h.svc.GetByID(c.Request.Context(), articleID)
	response.Success(c, toArticleResponse(article))
}

func (h *ArticleHandler) Delete(c *gin.Context) {
	userID := c.GetUint("user_id")
	role := c.GetString("role")
	articleID := getArticleID(c)

	err := h.svc.Delete(c.Request.Context(), userID, role, articleID)
	if err != nil {
		switch err {
		case service.ErrArticleNotFound:
			response.Error(c, errcode.ErrArticleNotFound)
		case service.ErrNotOwner:
			response.Error(c, errcode.ErrNotArticleOwner)
		default:
			response.Error(c, errcode.ErrInternal)
		}
		return
	}

	response.Success(c, nil)
}

func (h *ArticleHandler) ChangeStatus(c *gin.Context) {
	userID := c.GetUint("user_id")
	role := c.GetString("role")
	articleID := getArticleID(c)

	var req changeStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrBadRequest)
		return
	}

	err := h.svc.ChangeStatus(c.Request.Context(), userID, role, articleID, req.Status)
	if err != nil {
		switch err {
		case service.ErrArticleNotFound:
			response.Error(c, errcode.ErrArticleNotFound)
		case service.ErrNotOwner:
			response.Error(c, errcode.ErrNotArticleOwner)
		default:
			response.Error(c, errcode.ErrInvalidStatusChange)
		}
		return
	}

	response.Success(c, gin.H{"status": req.Status})
}

func getArticleID(c *gin.Context) uint {
	idStr := c.Param("id")
	id, _ := strconv.ParseUint(idStr, 10, 64)
	return uint(id)
}

func toArticleResponse(a *model.Article) dto.ArticleResponse {
	resp := dto.ArticleResponse{
		ID:        a.ID,
		AuthorID:  a.AuthorID,
		Title:     a.Title,
		Slug:      a.Slug,
		Content:   a.Content,
		Summary:   a.Summary,
		CoverURL:  a.CoverURL,
		Status:    a.Status,
		ViewCount: a.ViewCount,
		IsTop:     a.IsTop,
		CreatedAt: a.CreatedAt,
		UpdatedAt: a.UpdatedAt,
	}

	if a.Author.ID != 0 {
		resp.Author = dto.UserResponse{
			ID:       a.Author.ID,
			Username: a.Author.Username,
		}
	}

	for _, t := range a.Tags {
		resp.Tags = append(resp.Tags, dto.TagResponse{
			ID:   t.ID,
			Name: t.Name,
		})
	}

	if a.PublishedAt != nil {
		resp.PublishedAt = *a.PublishedAt
	}

	return resp
}
