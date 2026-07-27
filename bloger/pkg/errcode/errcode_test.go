package errcode

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrCode_Error_ReturnsMessage(t *testing.T) {
	err := ErrNotFound
	assert.Equal(t, "resource not found", err.Error())
}

func TestErrCode_HasHTTPStatus(t *testing.T) {
	tests := []struct {
		name string
		code *ErrCode
		want int
	}{
		{"internal error", ErrInternal, 500},
		{"bad request", ErrBadRequest, 400},
		{"not found", ErrNotFound, 404},
		{"unauthorized", ErrUnauthorized, 401},
		{"forbidden", ErrForbidden, 403},
		{"conflict", ErrConflict, 409},
		{"rate limited", ErrRateLimited, 429},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.code.HTTP)
		})
	}
}

func TestErrCode_CodesAreUnique(t *testing.T) {
	all := []*ErrCode{
		ErrInternal, ErrBadRequest, ErrNotFound,
		ErrUnauthorized, ErrForbidden, ErrConflict, ErrRateLimited,
		ErrInvalidCredentials, ErrEmailExists, ErrUsernameExists,
		ErrTokenExpired, ErrInvalidToken, ErrMissingAuthHeader, ErrInvalidAuthFormat,
		ErrArticleNotFound, ErrInvalidStatusChange, ErrNotArticleOwner,
		ErrCommentNotFound, ErrSensitiveWord,
	}

	seen := make(map[int]bool)
	for _, e := range all {
		assert.NotZero(t, e.Code, "ErrCode code must not be zero")
		assert.NotEmpty(t, e.Message, "ErrCode message must not be empty for code %d", e.Code)
		assert.False(t, seen[e.Code], "duplicate code: %d", e.Code)
		seen[e.Code] = true
	}
}

func TestErrCode_UserErrors(t *testing.T) {
	assert.Equal(t, 401, ErrInvalidCredentials.HTTP)
	assert.Equal(t, 409, ErrEmailExists.HTTP)
	assert.Equal(t, 409, ErrUsernameExists.HTTP)
	assert.Equal(t, 401, ErrTokenExpired.HTTP)
	assert.Equal(t, 401, ErrInvalidToken.HTTP)
	assert.Equal(t, 401, ErrMissingAuthHeader.HTTP)
	assert.Equal(t, 401, ErrInvalidAuthFormat.HTTP)
}

func TestErrCode_ArticleErrors(t *testing.T) {
	assert.Equal(t, 404, ErrArticleNotFound.HTTP)
	assert.Equal(t, 400, ErrInvalidStatusChange.HTTP)
	assert.Equal(t, 403, ErrNotArticleOwner.HTTP)
}

func TestErrCode_CommentErrors(t *testing.T) {
	assert.Equal(t, 404, ErrCommentNotFound.HTTP)
	assert.Equal(t, 400, ErrSensitiveWord.HTTP)
}
