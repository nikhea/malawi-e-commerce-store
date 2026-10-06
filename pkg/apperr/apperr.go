package apperr

import (
	"errors"
	"net/http"
)

// Code is a stable machine-readable error code. Handlers map it to an HTTP
// status via StatusOf; clients switch on Code, never on Message.
type Code string

const (
	CodeNotFound     Code = "NOT_FOUND"
	CodeValidation   Code = "VALIDATION_ERROR"
	CodeUnauthorized Code = "UNAUTHORIZED"
	CodeForbidden    Code = "FORBIDDEN"
	CodeConflict     Code = "CONFLICT"
	CodeInternal     Code = "INTERNAL"
)

// Error is the service-layer error type. Message is safe to expose to
// clients; Err carries the wrapped cause for server-side logs.
type Error struct {
	Code    Code
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return string(e.Code) + ": " + e.Message + ": " + e.Err.Error()
	}
	return string(e.Code) + ": " + e.Message
}

func (e *Error) Unwrap() error { return e.Err }

func NotFound(msg string) *Error     { return &Error{Code: CodeNotFound, Message: msg} }
func Validation(msg string) *Error   { return &Error{Code: CodeValidation, Message: msg} }
func Unauthorized(msg string) *Error { return &Error{Code: CodeUnauthorized, Message: msg} }
func Forbidden(msg string) *Error    { return &Error{Code: CodeForbidden, Message: msg} }
func Conflict(msg string) *Error     { return &Error{Code: CodeConflict, Message: msg} }

// Internal wraps an unexpected failure. err is kept for logs; only a
// generic message reaches the client.
func Internal(err error) *Error {
	return &Error{Code: CodeInternal, Message: "internal server error", Err: err}
}

// CodeOf unwraps err to its apperr Code, defaulting to CodeInternal for
// foreign errors (sql.ErrNoRows from a repo that forgot to map, nil panics, …).
func CodeOf(err error) Code {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return CodeInternal
}

// StatusOf maps an error to its HTTP status code.
func StatusOf(err error) int {
	switch CodeOf(err) {
	case CodeNotFound:
		return http.StatusNotFound
	case CodeValidation:
		return http.StatusBadRequest
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeConflict:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
