package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"seckill/pkg/errcode"
	"seckill/pkg/response"
)

// UserIDCtxKey 是注入到 gin.Context 的用户 ID 键。
const UserIDCtxKey = "user_id"

var errInvalidToken = errors.New("invalid token")

// Auth 校验请求头 X-Token（HMAC-SHA256 签名），提取真实 userID 注入 context。
// 不再信任客户端可伪造的 X-User-Id 头。
//
// Token 格式: base64url(payload).base64url(hmac(secret, base64url(payload)))
// payload: {"uid":<uint>,"exp":<unix 秒, 0 表示不过期>}
func Auth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("X-Token")
		if token == "" {
			response.Error(c, errcode.ErrForbidden)
			c.Abort()
			return
		}

		uid, err := parseToken(token, secret)
		if err != nil {
			response.Error(c, errcode.ErrForbidden)
			c.Abort()
			return
		}

		c.Set(UserIDCtxKey, uid)
		c.Next()
	}
}

// UserIDFromContext 从 gin.Context 提取已鉴权的用户 ID。
func UserIDFromContext(c *gin.Context) (uint, bool) {
	v, exists := c.Get(UserIDCtxKey)
	if !exists {
		return 0, false
	}
	uid, ok := v.(uint)
	return uid, ok
}

// 生成令牌（供登录/测试使用）。
func IssueToken(secret string, uid uint, exp int64) string {
	payload, _ := json.Marshal(struct {
		UID uint  `json:"uid"`
		Exp int64 `json:"exp"`
	}{UID: uid, Exp: exp})
	enc := base64.URLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(enc))
	sig := base64.URLEncoding.EncodeToString(mac.Sum(nil))
	return enc + "." + sig
}

func parseToken(token, secret string) (uint, error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return 0, errInvalidToken
	}

	sig, err := base64.URLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0, errInvalidToken
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(parts[0]))
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return 0, errInvalidToken
	}

	payload, err := base64.URLEncoding.DecodeString(parts[0])
	if err != nil {
		return 0, errInvalidToken
	}

	var p struct {
		UID uint  `json:"uid"`
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return 0, errInvalidToken
	}
	if p.UID == 0 {
		return 0, errInvalidToken
	}
	if p.Exp > 0 && time.Now().Unix() > p.Exp {
		return 0, errInvalidToken
	}
	return p.UID, nil
}
