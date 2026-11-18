package dashboard

import (
	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/internal/dashboard/handler"
	"github.com/nikhea/malawi-e-commerce-store/internal/dashboard/public"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/middleware"
)

// RegisterRoutes maps the admin dashboard surface. The route sits on the
// JWT-protected group and is additionally gated on the admin role. No query
// filters: the overview is a whole-store snapshot. Route table only, no logic.
func RegisterRoutes(protected *gin.RouterGroup, svc public.Service) {
	admin := protected.Group("/admin", middleware.RequireRole(string(userspublic.RoleAdmin)))
	admin.GET("/dashboard/overview", handler.New(svc).Overview)
}
