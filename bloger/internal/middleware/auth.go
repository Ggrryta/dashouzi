package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"bloger/pkg/errcode"
	"bloger/pkg/jwt"
	"bloger/pkg/response"
)

func Auth(j *jwt.JWT) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			response.Error(c, errcode.ErrMissingAuthHeader)
			c.Abort()
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			response.Error(c, errcode.ErrInvalidAuthFormat)
			c.Abort()
			return
		}

		tokenStr := parts[1]
		claims, err := j.ParseToken(tokenStr)
		if err != nil {
			if err == jwt.ErrTokenExpired {
				response.Error(c, errcode.ErrTokenExpired)
			} else {
				response.Error(c, errcode.ErrInvalidToken)
			}
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)

		c.Next()
	}
}
