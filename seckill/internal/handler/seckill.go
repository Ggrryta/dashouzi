package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"seckill/internal/middleware"
	"seckill/internal/service"
	"seckill/pkg/errcode"
	"seckill/pkg/response"
)

type SeckillHandler struct {
	svc *service.SeckillService
}

func NewSeckillHandler(svc *service.SeckillService) *SeckillHandler {
	return &SeckillHandler{svc: svc}
}

type buyReq struct {
	ItemID uint `json:"item_id" binding:"required"`
}

func (h *SeckillHandler) Buy(c *gin.Context) {
	// userID 由 Auth 中间件注入，不再信任可伪造的请求头
	userID, ok := middleware.UserIDFromContext(c)
	if !ok || userID == 0 {
		response.Error(c, errcode.ErrForbidden)
		return
	}

	var req buyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrBadRequest)
		return
	}

	result, err := h.svc.Execute(c.Request.Context(), req.ItemID, userID)
	if err != nil {
		response.Error(c, errcode.ErrInternal)
		return
	}

	switch result {
	case service.ResultSuccess:
		response.Success(c, gin.H{"result": "success"})
	case service.ResultSoldOut:
		response.Error(c, errcode.ErrSoldOut)
	case service.ResultAlreadyBought:
		response.Error(c, errcode.ErrAlreadyBought)
	case service.ResultSessionClosed:
		response.Error(c, errcode.ErrSessionClosed)
	}
}

func (h *SeckillHandler) Result(c *gin.Context) {
	itemID, err := strconv.ParseUint(c.Param("item_id"), 10, 64)
	if err != nil || itemID == 0 {
		response.Error(c, errcode.ErrBadRequest)
		return
	}
	userID, ok := middleware.UserIDFromContext(c)
	if !ok || userID == 0 {
		response.Error(c, errcode.ErrForbidden)
		return
	}

	bought, err := h.svc.IsBought(c.Request.Context(), uint(itemID), userID)
	if err != nil {
		response.Error(c, errcode.ErrInternal)
		return
	}
	stock, err := h.svc.GetStock(c.Request.Context(), uint(itemID))
	if err != nil {
		response.Error(c, errcode.ErrInternal)
		return
	}

	response.Success(c, gin.H{
		"item_id": itemID,
		"bought":  bought,
		"stock":   stock,
	})
}
