package jwt

import (
	"errors"
	"time"

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

func (j *JWT) GenerateToken(userID uint, username string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwtlib.RegisteredClaims{
			ExpiresAt: jwtlib.NewNumericDate(now.Add(time.Duration(j.expireHours) * time.Hour)),
			IssuedAt:  jwtlib.NewNumericDate(now),
		},
	}
	token := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)
	return token.SignedString(j.secret)
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
