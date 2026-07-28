package errcode

import "net/http"

type ErrCode struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	HTTP    int    `json:"-"`
}

var (
	ErrInternal    = &ErrCode{10000, "internal server error", http.StatusInternalServerError}
	ErrBadRequest  = &ErrCode{10001, "bad request", http.StatusBadRequest}
	ErrNotFound    = &ErrCode{10002, "resource not found", http.StatusNotFound}
	ErrUnauthorized = &ErrCode{10003, "unauthorized", http.StatusUnauthorized}
	ErrForbidden   = &ErrCode{10004, "forbidden", http.StatusForbidden}

	ErrInvalidToken   = &ErrCode{20001, "invalid or expired token", http.StatusUnauthorized}
	ErrUserExists     = &ErrCode{20002, "user already exists", http.StatusConflict}
	ErrInvalidLogin   = &ErrCode{20003, "invalid username or password", http.StatusUnauthorized}
)

func (e *ErrCode) Error() string { return e.Message }
