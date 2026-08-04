package middleware

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"auction/pkg/errcode"
	"auction/pkg/response"
)

const ContextUserIDKey = "user_id"

func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr := c.GetHeader("X-User-Id")
		if userIDStr == "" {
			response.Error(c, errcode.ErrUnauthorized)
			c.Abort()
			return
		}

		userID, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil || userID <= 0 {
			response.Error(c, errcode.ErrBadRequest)
			c.Abort()
			return
		}

		c.Set(ContextUserIDKey, userID)
		c.Next()
	}
}

func GetUserID(c *gin.Context) (int64, bool) {
	val, exists := c.Get(ContextUserIDKey)
	if !exists {
		return 0, false
	}
	userID, ok := val.(int64)
	return userID, ok
}

func MustGetUserID(c *gin.Context) int64 {
	userID, ok := GetUserID(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return 0
	}
	return userID
}
