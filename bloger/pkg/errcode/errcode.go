package errcode

import "net/http"

type ErrCode struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	HTTP    int    `json:"-"`
}

var (
	// 通用
	ErrInternal     = &ErrCode{10000, "internal server error", http.StatusInternalServerError}
	ErrBadRequest   = &ErrCode{10001, "bad request", http.StatusBadRequest}
	ErrNotFound     = &ErrCode{10002, "resource not found", http.StatusNotFound}
	ErrUnauthorized = &ErrCode{10003, "unauthorized", http.StatusUnauthorized}
	ErrForbidden    = &ErrCode{10004, "forbidden", http.StatusForbidden}
	ErrConflict     = &ErrCode{10005, "resource already exists", http.StatusConflict}
	ErrRateLimited  = &ErrCode{10006, "too many requests", http.StatusTooManyRequests}

	// 用户
	ErrInvalidCredentials = &ErrCode{20001, "invalid email or password", http.StatusUnauthorized}
	ErrEmailExists        = &ErrCode{20002, "email already exists", http.StatusConflict}
	ErrUsernameExists     = &ErrCode{20003, "username already exists", http.StatusConflict}
	ErrTokenExpired       = &ErrCode{20004, "token expired", http.StatusUnauthorized}
	ErrInvalidToken       = &ErrCode{20005, "invalid token", http.StatusUnauthorized}
	ErrMissingAuthHeader  = &ErrCode{20006, "missing authorization header", http.StatusUnauthorized}
	ErrInvalidAuthFormat  = &ErrCode{20007, "invalid authorization format", http.StatusUnauthorized}

	// 文章
	ErrArticleNotFound      = &ErrCode{30001, "article not found", http.StatusNotFound}
	ErrInvalidStatusChange  = &ErrCode{30002, "invalid status transition", http.StatusBadRequest}
	ErrNotArticleOwner      = &ErrCode{30003, "not the article owner", http.StatusForbidden}

	// 评论
	ErrCommentNotFound      = &ErrCode{40001, "comment not found", http.StatusNotFound}
	ErrSensitiveWord        = &ErrCode{40002, "content contains sensitive words", http.StatusBadRequest}
)

func (e *ErrCode) Error() string {
	return e.Message
}
