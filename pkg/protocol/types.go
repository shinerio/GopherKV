package protocol

import "net/http"

const (
	CodeOK             = 0
	CodeKeyNotFound    = 1001
	CodeKeyExpired     = 1002
	CodeKeyTooLong     = 2001
	CodeValueTooLarge  = 2002
	CodeInvalidRequest = 2003
	CodeMemoryFull     = 3001
	CodeInternal       = 5001
)

type APIResponse struct {
	Code int         `json:"code"`
	Data interface{} `json:"data"`
	Msg  string      `json:"msg"`
}

type APIError struct {
	Code       int
	Message    string
	HTTPStatus int
}

func (e *APIError) Error() string {
	return e.Message
}

func NewError(code int, msg string) *APIError {
	return &APIError{Code: code, Message: msg, HTTPStatus: HTTPStatusFromCode(code)}
}

func HTTPStatusFromCode(code int) int {
	switch code {
	case CodeOK:
		return http.StatusOK
	case CodeKeyNotFound, CodeKeyExpired:
		return http.StatusNotFound
	case CodeKeyTooLong, CodeValueTooLarge, CodeInvalidRequest:
		return http.StatusBadRequest
	case CodeMemoryFull:
		return http.StatusInsufficientStorage
	default:
		return http.StatusInternalServerError
	}
}
