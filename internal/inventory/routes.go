package inventory

import (
	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/internal/inventory/handler"
	"github.com/nikhea/malawi-e-commerce-store/internal/inventory/public"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/middleware"
)

// RegisterRoutes maps the inventory HTTP surface. Admin-only: stock is
// managed by the store, and reserve/release/confirm are service calls for
// orders/payments (this HTTP exists for ops, not for module traffic).
func RegisterRoutes(protected *gin.RouterGroup, svc public.Service) {
	h := handler.New(svc)

	admin := protected.Group("/admin", middleware.RequireRole(string(userspublic.RoleAdmin)))
	admin.GET("/inventory/stock", h.GetStock)
	admin.POST("/inventory/stock", h.SetStock)
	admin.POST("/inventory/reserve", h.Reserve)
	admin.POST("/inventory/release", h.Release)
	admin.POST("/inventory/confirm", h.Confirm)
}
