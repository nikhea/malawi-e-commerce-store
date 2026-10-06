package middleware

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORSConfig returns the storefront CORS policy: explicit origin
// allowlist (app + frontend), credentials allowed, tight methods and
// headers. Never "*" with credentials — that combination hands any
// site your users' JWT-authenticated responses.
func CORSConfig(appURL, frontendURL string) cors.Config {
	return cors.Config{
		AllowOrigins:     []string{appURL, frontendURL},
		AllowMethods:     []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Request-ID"},
		ExposeHeaders:    []string{"X-Request-ID", "Retry-After"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
}

// SecureHeaders sets response hardening for a JSON API:
//   - X-Content-Type-Options=nosniff blocks MIME-sniffing (stops a
//     JSON body being reinterpreted as HTML/JS — the XSS vector for
//     APIs whose output a browser might render).
//   - X-Frame-Options=DENY blocks clickjacking of API-driven pages.
//   - Referrer-Policy trims leaked URLs on outbound navigation.
//   - Content-Security-Policy "frame-ancestors 'none'" is the modern
//     complement to X-Frame-Options.
//
// Note on XSS: Go's encoding/json escapes <, >, & by default, so our
// envelope never emits raw markup. Stored content (review bodies, names)
// is the FRONTEND's job to escape on render — the API's contract is to
// never serve it as HTML, which JSON + nosniff guarantees.
func SecureHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Content-Security-Policy", "frame-ancestors 'none'")
		c.Next()
	}
}
