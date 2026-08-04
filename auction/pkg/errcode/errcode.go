package errcode

import "net/http"

type ErrCode struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	HTTP    int    `json:"-"`
}

func (e *ErrCode) Error() string {
	return e.Message
}

func (e *ErrCode) WithMessage(msg string) *ErrCode {
	return &ErrCode{Code: e.Code, Message: msg, HTTP: e.HTTP}
}

// 通用错误码: 0xxxx
var (
	Success        = &ErrCode{0, "ok", http.StatusOK}
	ErrInternal    = &ErrCode{10000, "internal server error", http.StatusInternalServerError}
	ErrBadRequest  = &ErrCode{10001, "bad request", http.StatusBadRequest}
	ErrNotFound    = &ErrCode{10002, "resource not found", http.StatusNotFound}
	ErrForbidden   = &ErrCode{10003, "forbidden", http.StatusForbidden}
	ErrConflict    = &ErrCode{10004, "resource conflict", http.StatusConflict}
	ErrRateLimited = &ErrCode{10005, "too many requests", http.StatusTooManyRequests}
	ErrUnauthorized = &ErrCode{10006, "unauthorized", http.StatusUnauthorized}
)

// 房间错误码: 100xxx
var (
	ErrRoomNameEmpty   = &ErrCode{100001, "room name cannot be empty", http.StatusBadRequest}
	ErrRoomNotFound    = &ErrCode{100002, "room not found", http.StatusNotFound}
	ErrRoomNotOwner    = &ErrCode{100003, "only room owner can perform this action", http.StatusForbidden}
	ErrRoomClosed      = &ErrCode{100004, "room is closed", http.StatusConflict}
)

// 商品错误码: 200xxx
var (
	ErrItemTitleEmpty       = &ErrCode{200001, "item title cannot be empty", http.StatusBadRequest}
	ErrItemPriceInvalid     = &ErrCode{200002, "start price or increment invalid", http.StatusBadRequest}
	ErrItemTimeInvalid      = &ErrCode{200003, "start time must be before end time", http.StatusBadRequest}
	ErrItemNotFound         = &ErrCode{200004, "item not found", http.StatusNotFound}
	ErrItemNotOwner         = &ErrCode{200005, "only item seller can perform this action", http.StatusForbidden}
	ErrItemCannotDelete     = &ErrCode{200006, "only pending item can be deleted", http.StatusConflict}
	ErrItemNotLive          = &ErrCode{200007, "item is not live", http.StatusConflict}
	ErrItemInvalidTransition = &ErrCode{200008, "invalid item status transition", http.StatusConflict}
)

// 出价错误码: 300xxx
var (
	ErrBidTooLow       = &ErrCode{300001, "bid amount is too low", http.StatusConflict}
	ErrBidTooFrequent  = &ErrCode{300002, "bid too frequently", http.StatusTooManyRequests}
	ErrBidItemClosed   = &ErrCode{300003, "item is closed for bidding", http.StatusConflict}
	ErrBidSelfForbidden = &ErrCode{300004, "seller cannot bid on own item", http.StatusForbidden}
)

// WebSocket 错误码: 400xxx
var (
	ErrWSInvalidMessage = &ErrCode{400001, "invalid websocket message", http.StatusBadRequest}
	ErrWSRoomNotFound   = &ErrCode{400002, "room not found", http.StatusNotFound}
)

// 系统错误码: 900xxx
var (
	ErrDBUnavailable    = &ErrCode{900001, "database unavailable", http.StatusServiceUnavailable}
	ErrRedisUnavailable = &ErrCode{900002, "redis unavailable", http.StatusServiceUnavailable}
)
