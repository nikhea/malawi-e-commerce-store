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
// CSRF note: tokens ride the Authorization header (never cookies), so
// browsers never attach them cross-origin — this API is CSRF-immune by
// construction. If cookie auth is ever added, pair it with SameSite +
// anti-CSRF tokens and revisit this claim.
func JWT(authSvc authpublic.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
		if !ok || strings.TrimSpace(token) == "" {
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
