package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	authpublic "github.com/nikhea/malawi-e-commerce-store/internal/auth/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	"github.com/nikhea/malawi-e-commerce-store/pkg/response"
)

// userIDKey is the Gin context key for the authenticated caller's id.
// Set here by JWT, read by handlers via UserIDOf.
const userIDKey = "user_id"

// JWT validates the Bearer token and publishes identity downstream:
// role via SetRole (for RequireRole) and user id via UserIDOf (for /me
// and future per-user scoping). It takes the auth PUBLIC interface, so
// this package never imports auth/service, auth/utils, or any repository.
//
// Prefix tolerance: "Bearer <token>" is canonical, but bare tokens are
// accepted too — Swagger UI's apiKey box sends the raw value with no
// prefix, and rejecting it only punishes tooling. The prefix carries no
// security (verification does), so tolerance costs nothing.
//
// CSRF note: tokens ride the Authorization header (never cookies), so
// browsers never attach them cross-origin — this API is CSRF-immune by
// construction. If cookie auth is ever added, pair it with SameSite +
// anti-CSRF tokens and revisit this claim.
func JWT(authSvc authpublic.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := bearerToken(c.GetHeader("Authorization"))
		if token == "" {
			response.Error(c, apperr.Unauthorized("authentication required"))
			c.Abort()
			return
		}
		claims, err := authSvc.Parse(token)
		if err != nil {
			response.Error(c, apperr.Unauthorized("invalid token"))
			c.Abort()
			return
		}
		SetRole(c, string(claims.Role))
		c.Set(userIDKey, claims.UserID)
		c.Next()
	}
}

// bearerToken extracts the JWT: "Bearer <token>" preferred, bare token
// tolerated (Swagger UI's apiKey box sends the raw value). A bare value
// must look like a JWT (three dot-separated segments, no spaces) —
// anything else is treated as missing, not as a token.
func bearerToken(header string) string {
	if t, ok := strings.CutPrefix(header, "Bearer "); ok {
		return strings.TrimSpace(t)
	}
	if t := strings.TrimSpace(header); strings.Count(t, ".") == 2 && !strings.Contains(t, " ") {
		return t
	}
	return ""
}

// UserIDOf returns the caller id stored by JWT, or "" when the middleware
// is not in the chain (e.g. handler unit tests).
func UserIDOf(c *gin.Context) string {
	if v, ok := c.Get(userIDKey); ok {
		if id, ok := v.(string); ok {
			return id
		}
	}
	return ""
}
