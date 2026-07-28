package errcode

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrCode_HasMessage(t *testing.T) {
	assert.NotEmpty(t, ErrInternal.Message)
	assert.NotEmpty(t, ErrBadRequest.Message)
	assert.NotEmpty(t, ErrSoldOut.Message)
	assert.NotEmpty(t, ErrAlreadyBought.Message)
}

func TestErrCode_CodesAreUnique(t *testing.T) {
	all := []*ErrCode{ErrInternal, ErrBadRequest, ErrNotFound, ErrForbidden, ErrConflict, ErrRateLimited, ErrSoldOut, ErrAlreadyBought, ErrSessionClosed}
	seen := make(map[int]bool)
	for _, e := range all {
		assert.False(t, seen[e.Code], "duplicate code: %d", e.Code)
		seen[e.Code] = true
	}
}

func TestErrCode_HTTPStatus(t *testing.T) {
	assert.Equal(t, 500, ErrInternal.HTTP)
	assert.Equal(t, 400, ErrBadRequest.HTTP)
	assert.Equal(t, 200, ErrSoldOut.HTTP)
	assert.Equal(t, 429, ErrRateLimited.HTTP)
}
