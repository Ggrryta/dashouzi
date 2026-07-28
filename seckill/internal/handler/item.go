package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"seckill/internal/repository"
	"seckill/internal/service"
	"seckill/pkg/errcode"
	"seckill/pkg/response"
)

type ItemHandler struct {
	svc   *service.ItemService
	redis repository.RedisClient
}

func NewItemHandler(svc *service.ItemService, redis repository.RedisClient) *ItemHandler {
	return &ItemHandler{svc: svc, redis: redis}
}

type createItemReq struct {
	SessionID   uint    `json:"session_id" binding:"required"`
	Title       string  `json:"title" binding:"required"`
	Price       float64 `json:"price" binding:"required"`
	OriginPrice float64 `json:"origin_price" binding:"required"`
	TotalStock  int     `json:"total_stock" binding:"required"`
	ImageURL    string  `json:"image_url"`
}

func (h *ItemHandler) Create(c *gin.Context) {
	var req createItemReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrBadRequest)
		return
	}

	item, err := h.svc.Create(c.Request.Context(), service.CreateItemInput{
		SessionID: req.SessionID, Title: req.Title,
		Price: req.Price, OriginPrice: req.OriginPrice,
		TotalStock: req.TotalStock, ImageURL: req.ImageURL,
	})
	if err != nil {
		response.ErrorWithMsg(c, errcode.ErrBadRequest, err.Error())
		return
	}
	response.Success(c, item)
}

func (h *ItemHandler) List(c *gin.Context) {
	items, err := h.svc.List(c.Request.Context())
	if err != nil {
		response.Error(c, errcode.ErrInternal)
		return
	}
	response.Success(c, items)
}

func (h *ItemHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	item, err := h.svc.GetByID(c.Request.Context(), uint(id))
	if err != nil || item == nil {
		response.Error(c, errcode.ErrNotFound)
		return
	}
	response.Success(c, item)
}

func (h *ItemHandler) BySession(c *gin.Context) {
	sessionID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	items, err := h.svc.FindBySession(c.Request.Context(), uint(sessionID))
	if err != nil {
		response.Error(c, errcode.ErrInternal)
		return
	}
	response.Success(c, items)
}

func (h *ItemHandler) Preheat(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	item, err := h.svc.GetByID(c.Request.Context(), uint(id))
	if err != nil || item == nil {
		response.Error(c, errcode.ErrNotFound)
		return
	}

	if err := service.PreheatStock(c.Request.Context(), h.redis, item); err != nil {
		response.Error(c, errcode.ErrInternal)
		return
	}

	response.Success(c, gin.H{"stock": item.TotalStock})
}
