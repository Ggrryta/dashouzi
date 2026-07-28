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

type CommentHandler struct {
	svc *service.CommentService
}

func NewCommentHandler(svc *service.CommentService) *CommentHandler {
	return &CommentHandler{svc: svc}
}

type createCommentReq struct {
	ArticleID uint  `json:"article_id" binding:"required"`
	ParentID  *uint `json:"parent_id"`
	Content   string `json:"content" binding:"required"`
}

func (h *CommentHandler) Create(c *gin.Context) {
	userID := c.GetUint("user_id")

	var req createCommentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrBadRequest)
		return
	}

	comment, err := h.svc.Create(c.Request.Context(), userID, req.ArticleID, service.CreateCommentInput{
		Content:  req.Content,
		ParentID: req.ParentID,
	})
	if err != nil {
		switch err {
		case service.ErrSensitiveContent:
			response.Error(c, errcode.ErrSensitiveWord)
		case service.ErrInvalidParent:
			response.Error(c, errcode.ErrInvalidParentComment)
		default:
			response.Error(c, errcode.ErrBadRequest)
		}
		return
	}

	response.Success(c, toCommentResponse(comment))
}

func (h *CommentHandler) Delete(c *gin.Context) {
	userID := c.GetUint("user_id")
	role := c.GetString("role")
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	err := h.svc.Delete(c.Request.Context(), userID, role, uint(id))
	if err != nil {
		switch err {
		case service.ErrCommentNotFound:
			response.Error(c, errcode.ErrCommentNotFound)
		case service.ErrNotCommentOwner:
			response.Error(c, errcode.ErrForbidden)
		default:
			response.Error(c, errcode.ErrInternal)
		}
		return
	}

	response.Success(c, nil)
}

func (h *CommentHandler) GetByArticle(c *gin.Context) {
	articleID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	comments, err := h.svc.GetByArticle(c.Request.Context(), uint(articleID))
	if err != nil {
		response.Error(c, errcode.ErrInternal)
		return
	}

	var result []dto.CommentResponse
	for _, c := range comments {
		result = append(result, toCommentTree(c))
	}

	response.Success(c, result)
}

func toCommentResponse(c *model.Comment) dto.CommentResponse {
	resp := dto.CommentResponse{
		ID:        c.ID,
		ArticleID: c.ArticleID,
		Content:   c.Content,
		CreatedAt: c.CreatedAt,
	}
	if c.User.ID != 0 {
		resp.User = dto.UserResponse{
			ID:       c.User.ID,
			Username: c.User.Username,
		}
	}
	if c.ParentID != nil {
		resp.ParentID = *c.ParentID
	}
	if c.IsDeleted {
		resp.Content = "[该评论已删除]"
	}
	return resp
}

func toCommentTree(c *model.Comment) dto.CommentResponse {
	resp := toCommentResponse(c)
	for _, r := range c.Replies {
		resp.Replies = append(resp.Replies, toCommentResponse(&r))
	}
	return resp
}
