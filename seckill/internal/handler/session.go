package handler

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"seckill/internal/service"
	"seckill/pkg/errcode"
	"seckill/pkg/response"
)

type SessionHandler struct {
	svc *service.SessionService
}

func NewSessionHandler(svc *service.SessionService) *SessionHandler {
	return &SessionHandler{svc: svc}
}

type createSessionReq struct {
	Name      string `json:"name" binding:"required"`
	StartTime string `json:"start_time" binding:"required"`
	EndTime   string `json:"end_time" binding:"required"`
}

func (h *SessionHandler) Create(c *gin.Context) {
	var req createSessionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrBadRequest)
		return
	}

	start, err := time.Parse("2006-01-02 15:04:05", req.StartTime)
	if err != nil {
		response.ErrorWithMsg(c, errcode.ErrBadRequest, "invalid start_time format")
		return
	}
	end, err := time.Parse("2006-01-02 15:04:05", req.EndTime)
	if err != nil {
		response.ErrorWithMsg(c, errcode.ErrBadRequest, "invalid end_time format")
		return
	}

	session, err := h.svc.Create(c.Request.Context(), service.CreateSessionInput{
		Name: req.Name, StartTime: start, EndTime: end,
	})
	if err != nil {
		response.ErrorWithMsg(c, errcode.ErrBadRequest, err.Error())
		return
	}

	response.Success(c, session)
}

func (h *SessionHandler) List(c *gin.Context) {
	sessions, err := h.svc.List(c.Request.Context())
	if err != nil {
		response.Error(c, errcode.ErrInternal)
		return
	}
	response.Success(c, sessions)
}

func (h *SessionHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	session, err := h.svc.GetByID(c.Request.Context(), uint(id))
	if err != nil || session == nil {
		response.Error(c, errcode.ErrNotFound)
		return
	}
	response.Success(c, session)
}
