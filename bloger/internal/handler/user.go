package handler

import (
	"github.com/gin-gonic/gin"

	"bloger/internal/dto"
	"bloger/internal/model"
	"bloger/internal/service"
	"bloger/pkg/errcode"
	"bloger/pkg/jwt"
	"bloger/pkg/response"
)

type UserHandler struct {
	svc *service.UserService
	jwt *jwt.JWT
}

func NewUserHandler(svc *service.UserService, j *jwt.JWT) *UserHandler {
	return &UserHandler{svc: svc, jwt: j}
}

func (h *UserHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrBadRequest)
		return
	}

	user, err := h.svc.Register(c.Request.Context(), service.RegisterInput{
		Username: req.Username,
		Email:    req.Email,
		Password: req.Password,
	})

	if err != nil {
		switch err {
		case service.ErrEmailExists:
			response.Error(c, errcode.ErrEmailExists)
		case service.ErrUsernameExists:
			response.Error(c, errcode.ErrUsernameExists)
		default:
			response.Error(c, errcode.ErrBadRequest)
		}
		return
	}

	response.Success(c, dto.RegisterResponse{
		User: toUserResponse(user),
	})
}

func (h *UserHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrBadRequest)
		return
	}

	user, err := h.svc.Login(c.Request.Context(), service.LoginInput{
		Email:    req.Email,
		Password: req.Password,
	})

	if err != nil {
		switch err {
		case service.ErrInvalidCredentials:
			response.Error(c, errcode.ErrInvalidCredentials)
		default:
			response.Error(c, errcode.ErrBadRequest)
		}
		return
	}

	token, err := h.jwt.GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		response.Error(c, errcode.ErrInternal)
		return
	}

	response.Success(c, dto.LoginResponse{
		Token: token,
		User:  toUserResponse(user),
	})
}

func (h *UserHandler) GetMe(c *gin.Context) {
	userID := c.GetUint("user_id")

	user, err := h.svc.GetByID(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, errcode.ErrNotFound)
		return
	}

	response.Success(c, toUserResponse(user))
}

func toUserResponse(u *model.User) dto.UserResponse {
	return dto.UserResponse{
		ID:        u.ID,
		Username:  u.Username,
		Email:     u.Email,
		Role:      u.Role,
		AvatarURL: u.AvatarURL,
		Bio:       u.Bio,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}
