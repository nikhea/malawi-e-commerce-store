package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
)

// Envelope shapes. Success: {"data": …}. Failure: {"error": {"code": …, "message": …}}.
type envelope struct {
	Data any `json:"data,omitempty"`
}

type errorBody struct {
	Code    apperr.Code `json:"code"`
	Message string      `json:"message"`
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

// OK writes 200 with {"data": data}.
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, envelope{Data: data})
}

// Created writes 201 with {"data": data}.
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, envelope{Data: data})
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
	c.JSON(apperr.StatusOf(err), errorEnvelope{
		Error: errorBody{Code: code, Message: msg},
	})
}
