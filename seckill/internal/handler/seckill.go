package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

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
	userIDStr := c.GetHeader("X-User-Id")
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil || userID == 0 {
		response.Error(c, errcode.ErrBadRequest)
		return
	}

	var req buyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrBadRequest)
		return
	}

	result, err := h.svc.Execute(c.Request.Context(), req.ItemID, uint(userID))
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
	}
}

func (h *SeckillHandler) Result(c *gin.Context) {
	itemID, _ := strconv.ParseUint(c.Param("item_id"), 10, 64)
	userIDStr := c.GetHeader("X-User-Id")
	userID, _ := strconv.ParseUint(userIDStr, 10, 64)

	bought, _ := h.svc.IsBought(c.Request.Context(), uint(itemID), uint(userID))
	stock, _ := h.svc.GetStock(c.Request.Context(), uint(itemID))

	response.Success(c, gin.H{
		"item_id": itemID,
		"bought":  bought,
		"stock":   stock,
	})
}
