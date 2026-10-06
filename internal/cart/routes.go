package cart

import (
	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/internal/cart/handler"
	"github.com/nikhea/malawi-e-commerce-store/internal/cart/public"
)

// RegisterRoutes maps the cart HTTP surface. All routes live on the
// JWT-protected group (identity comes from the token); no admin surface.
// Route table only, no logic.
func RegisterRoutes(protected *gin.RouterGroup, svc public.Service) {
	h := handler.New(svc)

	cart := protected.Group("/cart")
	cart.GET("", h.Get)
	cart.POST("/items", h.AddItem)
	cart.PATCH("/items/:id", h.SetQty)
	cart.DELETE("/items/:id", h.RemoveItem)
	cart.DELETE("", h.Clear)
}
