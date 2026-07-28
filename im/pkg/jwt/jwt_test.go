package jwt

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGenerateAndParse(t *testing.T) {
	j := New("secret", 24)
	token, err := j.GenerateToken(42, "alice")
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	claims, err := j.ParseToken(token)
	assert.NoError(t, err)
	assert.Equal(t, uint(42), claims.UserID)
	assert.Equal(t, "alice", claims.Username)
}

func TestParse_ExpiredToken(t *testing.T) {
	j := New("secret", -1) // 立即过期
	token, _ := j.GenerateToken(1, "test")
	time.Sleep(10 * time.Millisecond)
	_, err := j.ParseToken(token)
	assert.ErrorIs(t, err, ErrTokenExpired)
}

func TestParse_InvalidToken(t *testing.T) {
	j := New("secret", 24)
	_, err := j.ParseToken("bad.token.here")
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestParse_EmptyToken(t *testing.T) {
	j := New("secret", 24)
	_, err := j.ParseToken("")
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestParse_WrongSecret(t *testing.T) {
	j1 := New("secret-a", 24)
	j2 := New("secret-b", 24)
	token, _ := j1.GenerateToken(1, "test")
	_, err := j2.ParseToken(token)
	assert.ErrorIs(t, err, ErrInvalidToken)
}
