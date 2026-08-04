package handler

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"auction/internal/middleware"
	"auction/internal/model"
	"auction/internal/service"
	"auction/pkg/errcode"
	"auction/pkg/response"
)

// ItemHandler 商品 handler
type ItemHandler struct {
	itemService service.ItemService
}

// NewItemHandler 创建商品 handler
func NewItemHandler(itemService service.ItemService) *ItemHandler {
	return &ItemHandler{itemService: itemService}
}

// CreateItemRequest 创建商品请求
type CreateItemRequest struct {
	Title        string `json:"title" binding:"required,max=128"`
	Description  string `json:"description" binding:"max=2048"`
	ImageURL     string `json:"imageUrl" binding:"omitempty,max=512,url"`
	StartPrice   int64  `json:"startPrice" binding:"required,min=1"`
	MinIncrement int64  `json:"minIncrement" binding:"required,min=1"`
	StartTime    string `json:"startTime" binding:"required"`
	EndTime      string `json:"endTime" binding:"required"`
}

// ItemResponse 商品响应
type ItemResponse struct {
	ID           int64            `json:"id"`
	RoomID       int64            `json:"roomId"`
	SellerID     int64            `json:"sellerId"`
	Title        string           `json:"title"`
	Description  string           `json:"description"`
	ImageURL     string           `json:"imageUrl"`
	StartPrice   int64            `json:"startPrice"`
	MinIncrement int64            `json:"minIncrement"`
	Status       model.ItemStatus `json:"status"`
	StartTime    string           `json:"startTime"`
	EndTime      string           `json:"endTime"`
	CurrentPrice int64            `json:"currentPrice"`
	NextMinBid   int64            `json:"nextMinBid"`
	BidCount     int64            `json:"bidCount"`
	WinnerID     *int64           `json:"winnerId"`
	CreatedAt    string           `json:"createdAt"`
}

// Create 创建商品
func (h *ItemHandler) Create(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	roomID, err := strconv.ParseInt(c.Param("roomId"), 10, 64)
	if err != nil || roomID <= 0 {
		response.Error(c, errcode.ErrBadRequest.WithMessage("房间 ID 无效"))
		return
	}

	var req CreateItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrBadRequest.WithMessage(err.Error()))
		return
	}

	startTime, endTime, err := parseItemTime(req.StartTime, req.EndTime)
	if err != nil {
		response.Error(c, errcode.ErrBadRequest.WithMessage(err.Error()))
		return
	}

	item, err := h.itemService.Create(c.Request.Context(), &service.CreateItemRequest{
		RoomID:       roomID,
		SellerID:     userID,
		Title:        req.Title,
		Description:  req.Description,
		ImageURL:     req.ImageURL,
		StartPrice:   req.StartPrice,
		MinIncrement: req.MinIncrement,
		StartTime:    startTime,
		EndTime:      endTime,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, itemToResponse(item))
}

// Get 查询商品详情
func (h *ItemHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("itemId"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, errcode.ErrBadRequest.WithMessage("商品 ID 无效"))
		return
	}

	item, err := h.itemService.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, itemToResponse(item))
}

// ListByRoom 查询房间内商品列表
func (h *ItemHandler) ListByRoom(c *gin.Context) {
	roomID, err := strconv.ParseInt(c.Param("roomId"), 10, 64)
	if err != nil || roomID <= 0 {
		response.Error(c, errcode.ErrBadRequest.WithMessage("房间 ID 无效"))
		return
	}

	status := model.ItemStatus(c.Query("status"))
	cursor, _ := strconv.ParseInt(c.Query("cursor"), 10, 64)
	size, _ := strconv.Atoi(c.Query("size"))

	items, err := h.itemService.ListByRoom(c.Request.Context(), roomID, status, cursor, size)
	if err != nil {
		response.Error(c, err)
		return
	}

	list := make([]*ItemResponse, 0, len(items))
	for _, item := range items {
		list = append(list, itemToResponse(item))
	}
	response.Success(c, list)
}

// Delete 删除商品
func (h *ItemHandler) Delete(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	id, err := strconv.ParseInt(c.Param("itemId"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, errcode.ErrBadRequest.WithMessage("商品 ID 无效"))
		return
	}

	if err := h.itemService.Delete(c.Request.Context(), id, userID); err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, nil)
}

func parseItemTime(start, end string) (time.Time, time.Time, error) {
	const layout = "2006-01-02T15:04:05.000Z"
	startTime, err := time.Parse(layout, start)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	endTime, err := time.Parse(layout, end)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return startTime, endTime, nil
}

func itemToResponse(item *model.Item) *ItemResponse {
	nextMinBid := item.StartPrice
	if item.CurrentPrice > 0 {
		nextMinBid = item.CurrentPrice + item.MinIncrement
	}

	return &ItemResponse{
		ID:           item.ID,
		RoomID:       item.RoomID,
		SellerID:     item.SellerID,
		Title:        item.Title,
		Description:  item.Description,
		ImageURL:     item.ImageURL,
		StartPrice:   item.StartPrice,
		MinIncrement: item.MinIncrement,
		Status:       item.Status,
		StartTime:    item.StartTime.Format("2006-01-02T15:04:05.000Z"),
		EndTime:      item.EndTime.Format("2006-01-02T15:04:05.000Z"),
		CurrentPrice: item.CurrentPrice,
		NextMinBid:   nextMinBid,
		BidCount:     item.BidCount,
		WinnerID:     item.WinnerID,
		CreatedAt:    item.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
	}
}
