package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/internal/auth/handler"
	"github.com/nikhea/malawi-e-commerce-store/internal/auth/public"
)

// RegisterRoutes maps the auth HTTP surface. Open group — register/login
// must NOT sit behind the JWT middleware. Route table only, no logic.
func RegisterRoutes(g *gin.RouterGroup, svc public.Service) {
	h := handler.New(svc)

	auth := g.Group("/auth")
	auth.POST("/register", h.Register)
	auth.POST("/login", h.Login)
}
