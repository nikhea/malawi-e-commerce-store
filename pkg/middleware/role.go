package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	"github.com/nikhea/malawi-e-commerce-store/pkg/response"
)

// roleKey is the Gin context key under which the JWT middleware (Phase 3,
// auth) stores the caller's role string. Kept here — not in auth — so this
// file never imports a module and no import cycle is possible.
const roleKey = "role"

// SetRole stores the caller's role for RequireRole to check. Called by the
// JWT middleware after validating the token.
func SetRole(c *gin.Context, role string) {
	c.Set(roleKey, role)
}

// RequireRole gates a route group to the given roles. No role in context
// (missing/invalid token) → 401; wrong role → 403.
func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}
	return func(c *gin.Context) {
		v, ok := c.Get(roleKey)
		role, isString := v.(string)
		if !ok || !isString || role == "" {
			response.Error(c, apperr.Unauthorized("authentication required"))
			c.Abort()
			return
		}
		if _, ok := allowed[role]; !ok {
			response.Error(c, apperr.Forbidden("admin access required"))
			c.Abort()
			return
		}
		c.Next()
	}
}
