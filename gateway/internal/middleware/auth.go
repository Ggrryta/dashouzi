package middleware

import (
	"net/http"
	"fmt"
	"strings"

	"gateway/pkg/jwt"
)

// Auth 返回一个 HTTP 中间件，校验 JWT 并将 user_id 注入 Header 传给上游
func Auth(j *jwt.JWT, whitelist []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 白名单免认证
			for _, p := range whitelist {
				if strings.HasPrefix(r.URL.Path, p) {
					next.ServeHTTP(w, r)
					return
				}
			}

			header := r.Header.Get("Authorization")
			if header == "" {
				http.Error(w, `{"code":20006,"message":"missing auth"}`, http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				http.Error(w, `{"code":20007,"message":"bad format"}`, http.StatusUnauthorized)
				return
			}

			claims, err := j.ParseToken(parts[1])
			if err != nil {
				http.Error(w, `{"code":20005,"message":"invalid token"}`, http.StatusUnauthorized)
				return
			}

			r.Header.Set("X-User-Id", fmt.Sprintf("%d", claims.UserID))
			next.ServeHTTP(w, r)
		})
	}
}
