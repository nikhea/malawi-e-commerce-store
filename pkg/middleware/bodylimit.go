package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Request-body caps applied globally before any handler binds input.
// JSON endpoints are small; multipart uploads legitimately carry up to
// 20 × 10 MiB of images (products handler), so they get their own cap.
const (
	DefaultJSONBodyLimit      = 1 << 20   // 1 MiB
	DefaultMultipartBodyLimit = 210 << 20 // 210 MiB (20 images + form overhead)
)

// BodyLimit wraps the request body in http.MaxBytesReader. Chosen by
// content type so the generous upload allowance cannot be used to stream
// arbitrary large JSON bodies into memory.
func BodyLimit(jsonLimit, multipartLimit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body == nil {
			c.Next()
			return
		}
		limit := jsonLimit
		if strings.HasPrefix(strings.ToLower(c.GetHeader("Content-Type")), "multipart/") {
			limit = multipartLimit
		}
		if limit > 0 {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		}
		c.Next()
	}
}
