package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
)

// Envelope shapes. Success: {"data": …}. Paged success adds a sibling
// "meta" object so existing clients keep reading an array from data.
// Failure: {"error": {"code": …, "message": …}}.
type envelope struct {
	Data any `json:"data,omitempty"`
	Meta any `json:"meta,omitempty"`
}

// ErrorDetail is one machine-readable failure. Exported so swag can
// reference it in generated API docs.
type ErrorDetail struct {
	Code    apperr.Code `json:"code" example:"NOT_FOUND"`
	Message string      `json:"message" example:"user not found"`
}

// ErrorResponse is the failure envelope. Exported so swag can reference
// it in generated API docs.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// OK writes 200 with {"data": data}.
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, envelope{Data: data})
}

// OKMeta writes 200 with {"data": data, "meta": meta}. List endpoints use
// it: data stays the row array (backward compatible) and meta carries
// page/limit/total counters.
func OKMeta(c *gin.Context, data any, meta any) {
	c.JSON(http.StatusOK, envelope{Data: data, Meta: meta})
}

// Created writes 201 with {"data": data}.
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, envelope{Data: data})
}

// Accepted writes 202 with {"data": data}. For async work (River jobs):
// the request is valid and queued, the outcome comes later.
func Accepted(c *gin.Context, data any) {
	c.JSON(http.StatusAccepted, envelope{Data: data})
}

// NoContent writes 204 with an empty body.
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// Error writes {"error": {"code", "message"}} with the status mapped from
// the apperr code. Foreign (non-apperr) errors become 500 INTERNAL with a
// generic message so internals never leak to clients.
func Error(c *gin.Context, err error) {
	code := apperr.CodeOf(err)
	msg := "internal server error"
	var e *apperr.Error
	if errors.As(err, &e) && e.Message != "" && code != apperr.CodeInternal {
		msg = e.Message
	}
	c.JSON(apperr.StatusOf(err), ErrorResponse{
		Error: ErrorDetail{Code: code, Message: msg},
	})
}
