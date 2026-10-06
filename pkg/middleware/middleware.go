package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// requestIDKey is the Gin context + header key for the request id.
const requestIDKey = "X-Request-ID"

// RequestID assigns each request a UUID (or reuses the caller's
// X-Request-ID header), exposes it as a response header, and stores it in
// the Gin context so handlers and services can include it in logs.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(requestIDKey)
		if id == "" {
			id = uuid.NewString()
		}
		c.Set(requestIDKey, id)
		c.Header(requestIDKey, id)
		c.Next()
	}
}

// RequestIDOf returns the request id stored by RequestID, or "" if the
// middleware is not installed (e.g. in handler unit tests).
func RequestIDOf(c *gin.Context) string {
	if v, ok := c.Get(requestIDKey); ok {
		if id, ok := v.(string); ok {
			return id
		}
	}
	return ""
}

// Logger writes one line per request: request id, method, path, status,
// latency. stdlib log keeps the foundation dependency-free; swap for slog
// or zap when log volume says so.
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Printf("request_id=%s method=%s path=%s status=%d latency=%s",
			RequestIDOf(c), c.Request.Method, c.FullPath(), c.Writer.Status(), time.Since(start))
	}
}
