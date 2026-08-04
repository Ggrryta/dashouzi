package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"auction/internal/middleware"
	"auction/internal/model"
	"auction/internal/service"
	"auction/pkg/errcode"
	"auction/pkg/response"
)

// RoomHandler 房间 handler
type RoomHandler struct {
	roomService service.RoomService
}

// NewRoomHandler 创建房间 handler
func NewRoomHandler(roomService service.RoomService) *RoomHandler {
	return &RoomHandler{roomService: roomService}
}

// CreateRoomRequest 创建房间请求
type CreateRoomRequest struct {
	Name        string `json:"name" binding:"required,max=128"`
	Description string `json:"description" binding:"max=2048"`
}

// CreateRoomResponse 创建房间响应
type CreateRoomResponse struct {
	ID          int64             `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	OwnerID     int64             `json:"ownerId"`
	Status      model.RoomStatus  `json:"status"`
	CreatedAt   string            `json:"createdAt"`
}

// Create 创建房间
func (h *RoomHandler) Create(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	var req CreateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrBadRequest.WithMessage(err.Error()))
		return
	}

	room, err := h.roomService.Create(c.Request.Context(), userID, req.Name, req.Description)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, roomToResponse(room))
}

// Get 查询房间详情
func (h *RoomHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("roomId"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, errcode.ErrBadRequest.WithMessage("房间 ID 无效"))
		return
	}

	room, err := h.roomService.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, roomToResponse(room))
}

// List 查询房间列表
func (h *RoomHandler) List(c *gin.Context) {
	cursor, _ := strconv.ParseInt(c.Query("cursor"), 10, 64)
	size, _ := strconv.Atoi(c.Query("size"))

	rooms, err := h.roomService.List(c.Request.Context(), cursor, size)
	if err != nil {
		response.Error(c, err)
		return
	}

	list := make([]*CreateRoomResponse, 0, len(rooms))
	for _, room := range rooms {
		list = append(list, roomToResponse(room))
	}
	response.Success(c, list)
}

// Close 关闭房间
func (h *RoomHandler) Close(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	id, err := strconv.ParseInt(c.Param("roomId"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, errcode.ErrBadRequest.WithMessage("房间 ID 无效"))
		return
	}

	if err := h.roomService.Close(c.Request.Context(), id, userID); err != nil {
		response.Error(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func roomToResponse(room *model.Room) *CreateRoomResponse {
	return &CreateRoomResponse{
		ID:          room.ID,
		Name:        room.Name,
		Description: room.Description,
		OwnerID:     room.OwnerID,
		Status:      room.Status,
		CreatedAt:   room.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
	}
}
