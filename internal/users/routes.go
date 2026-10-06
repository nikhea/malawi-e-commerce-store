package users

import (
	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/internal/users/handler"
	"github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/middleware"
)

// RegisterRoutes maps the users HTTP surface. Route table only — no logic.
// The JWT middleware (Phase 3, auth) authenticates and sets the role;
// RequireRole then gates the admin group. Until Phase 3 lands, /admin/*
// answers 401.
// Single storefront: admin manages the one store's users directly here.
func RegisterRoutes(g *gin.RouterGroup, svc public.Service) {
	h := handler.New(svc)

	users := g.Group("/users")
	users.GET("/:id", h.GetByID)

	admin := g.Group("/admin", middleware.RequireRole(string(public.RoleAdmin)))
	admin.GET("/users/:id", h.GetByID)
	admin.PATCH("/users/:id/role", h.SetRole)
}
