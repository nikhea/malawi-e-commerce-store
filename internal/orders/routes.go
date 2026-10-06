package orders

import (
	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/internal/orders/handler"
	"github.com/nikhea/malawi-e-commerce-store/internal/orders/public"
)

// RegisterRoutes maps the orders HTTP surface. All routes live on the
// JWT-protected group; ownership is enforced in the service (owner id
// from the token, never from the client). Route table only, no logic.
func RegisterRoutes(protected *gin.RouterGroup, svc public.Service) {
	h := handler.New(svc)

	orders := protected.Group("/orders")
	orders.POST("/checkout", h.Checkout)
	orders.GET("", h.ListMine)
	orders.GET("/:id", h.GetByID)
	orders.POST("/:id/cancel", h.Cancel)
}
