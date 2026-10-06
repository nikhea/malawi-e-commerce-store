package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/internal/auth/handler"
	"github.com/nikhea/malawi-e-commerce-store/internal/auth/public"
)

// RegisterRoutes maps the auth HTTP surface. Takes the pre-scoped /auth
// group (the wiring site attaches the brute-force rate limiter here) —
// still open, never behind JWT. Route table only, no logic.
func RegisterRoutes(auth *gin.RouterGroup, svc public.Service) {
	h := handler.New(svc)

	auth.POST("/register", h.Register)
	auth.POST("/login", h.Login)
	auth.POST("/refresh", h.Refresh)
	auth.POST("/logout", h.Logout)
	auth.POST("/verify-email/request", h.RequestVerification)
	auth.POST("/verify-email", h.VerifyEmail)
	auth.POST("/password/forgot", h.RequestPasswordReset)
	auth.POST("/password/reset", h.ResetPassword)
}
