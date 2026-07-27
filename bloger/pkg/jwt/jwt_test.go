package jwt

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGenerateToken_ReturnsNonEmptyToken(t *testing.T) {
	j := New("test-secret", 24)

	token, err := j.GenerateToken(1, "admin", "admin")
	assert.NoError(t, err)
	assert.NotEmpty(t, token)
}

func TestParseToken_ReturnsCorrectClaims(t *testing.T) {
	j := New("test-secret", 24)
	token, _ := j.GenerateToken(42, "alice", "author")

	claims, err := j.ParseToken(token)
	assert.NoError(t, err)
	assert.Equal(t, uint(42), claims.UserID)
	assert.Equal(t, "alice", claims.Username)
	assert.Equal(t, "author", claims.Role)
}

func TestParseToken_ExpiredToken(t *testing.T) {
	j := New("test-secret", -1) // 立即过期

	token, err := j.GenerateToken(1, "test", "reader")
	assert.NoError(t, err)

	// 等 token 过期
	time.Sleep(100 * time.Millisecond)

	_, err = j.ParseToken(token)
	assert.Error(t, err)
}

func TestParseToken_InvalidToken(t *testing.T) {
	j := New("test-secret", 24)

	_, err := j.ParseToken("not.a.valid.token")
	assert.Error(t, err)
}

func TestParseToken_EmptyToken(t *testing.T) {
	j := New("test-secret", 24)

	_, err := j.ParseToken("")
	assert.Error(t, err)
}

func TestParseToken_WrongSecret(t *testing.T) {
	j1 := New("secret-a", 24)
	j2 := New("secret-b", 24)

	token, _ := j1.GenerateToken(1, "test", "reader")
	_, err := j2.ParseToken(token)
	assert.Error(t, err)
}

func TestParseToken_TamperedToken(t *testing.T) {
	j := New("test-secret", 24)
	token, _ := j.GenerateToken(1, "test", "reader")

	// 篡改 token 的 payload 部分
	tampered := token + "x"
	_, err := j.ParseToken(tampered)
	assert.Error(t, err)
}

func TestParseToken_AlgNone(t *testing.T) {
	j := New("test-secret", 24)
	// 构造一个 alg=none 的 token
	_, err := j.ParseToken("eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJ1c2VyX2lkIjoxfQ.")
	assert.Error(t, err)
}
