package errcode

import "net/http"

type ErrCode struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	HTTP    int    `json:"-"`
}

var (
	ErrInternal     = &ErrCode{10000, "internal server error", http.StatusInternalServerError}
	ErrBadRequest   = &ErrCode{10001, "bad request", http.StatusBadRequest}
	ErrNotFound     = &ErrCode{10002, "resource not found", http.StatusNotFound}
	ErrForbidden    = &ErrCode{10004, "forbidden", http.StatusForbidden}
	ErrConflict     = &ErrCode{10005, "resource already exists", http.StatusConflict}
	ErrRateLimited  = &ErrCode{10006, "too many requests", http.StatusTooManyRequests}

	// 秒杀
	ErrSoldOut       = &ErrCode{50001, "sold out", http.StatusOK}
	ErrAlreadyBought = &ErrCode{50002, "already bought", http.StatusOK}
	ErrSessionClosed = &ErrCode{50003, "seckill session is not active", http.StatusBadRequest}
)

func (e *ErrCode) Error() string {
	return e.Message
}
