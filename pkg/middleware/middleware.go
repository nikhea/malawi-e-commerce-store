package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// requestIDKey is the Gin context + header key for the request id.
const requestIDKey = "X-Request-ID"

// maxRequestIDLen bounds the caller-supplied id we echo and log.
const maxRequestIDLen = 64

// sanitizeRequestID keeps only characters that are safe in a log line and
// an HTTP header. A caller-supplied id with newlines could forge log
// entries; an unbounded one could bloat every line.
func sanitizeRequestID(id string) string {
	if len(id) > maxRequestIDLen {
		id = id[:maxRequestIDLen]
	}
	var b string
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b += string(r)
		case r == '-', r == '_', r == '.', r == ':', r == '/':
			b += string(r)
		}
	}
	return b
}

// RequestID assigns each request a UUID (or reuses the caller's
// X-Request-ID header), exposes it as a response header, and stores it in
// the Gin context so handlers and services can include it in logs.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := sanitizeRequestID(c.GetHeader(requestIDKey))
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
