package orders

import (
	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/internal/orders/handler"
	"github.com/nikhea/malawi-e-commerce-store/internal/orders/public"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/middleware"
)

// RegisterRoutes maps the orders HTTP surface. All routes live on the
// JWT-protected group; ownership is enforced in the service (owner id
// from the token, never from the client). The admin aggregate is global,
// optionally narrowed by user_id, and sits behind RequireRole. Route table
// only, no logic.
func RegisterRoutes(protected *gin.RouterGroup, svc public.Service) {
	h := handler.New(svc)

	orders := protected.Group("/orders")
	orders.POST("/checkout", h.Checkout)
	orders.GET("", h.ListMine)
	// Static segments must be registered before /:id or Gin routes the
	// request to GetByID with id="aggregates".
	orders.GET("/aggregates", h.AggregatesMine)
	orders.GET("/:id", h.GetByID)
	orders.POST("/:id/cancel", h.Cancel)

	admin := protected.Group("/admin", middleware.RequireRole(string(userspublic.RoleAdmin)))
	admin.GET("/orders/aggregates", h.AggregatesAll)
}
