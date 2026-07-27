package middleware

import (
	"github.com/gin-gonic/gin"

	"bloger/pkg/errcode"
	"bloger/pkg/response"
)

// 角色层级：admin > author > reader
var roleHierarchy = map[string]int{
	"reader": 1,
	"author": 2,
	"admin":  3,
}

func Role(minRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			response.Error(c, errcode.ErrForbidden)
			c.Abort()
			return
		}

		roleStr, ok := role.(string)
		if !ok {
			response.Error(c, errcode.ErrForbidden)
			c.Abort()
			return
		}

		if !hasAccess(roleStr, minRole) {
			response.Error(c, errcode.ErrForbidden)
			c.Abort()
			return
		}

		c.Next()
	}
}

func hasAccess(userRole, requiredRole string) bool {
	userLevel, uok := roleHierarchy[userRole]
	requiredLevel, rok := roleHierarchy[requiredRole]
	if !uok || !rok {
		return false
	}
	return userLevel >= requiredLevel
}
