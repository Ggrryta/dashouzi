package jwt

import (
	"errors"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

var (
	ErrTokenExpired = errors.New("token expired")
	ErrInvalidToken = errors.New("invalid token")
)

type JWT struct {
	secret      []byte
	expireHours int
}

type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	jwtlib.RegisteredClaims
}

func New(secret string, expireHours int) *JWT {
	return &JWT{secret: []byte(secret), expireHours: expireHours}
}

func (j *JWT) ParseToken(tokenStr string) (*Claims, error) {
	if tokenStr == "" {
		return nil, ErrInvalidToken
	}
	token, err := jwtlib.ParseWithClaims(tokenStr, &Claims{}, func(t *jwtlib.Token) (interface{}, error) {
		return j.secret, nil
	})
	if err != nil {
		if errors.Is(err, jwtlib.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
