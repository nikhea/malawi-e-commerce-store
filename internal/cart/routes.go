package cart

import (
	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/internal/cart/handler"
	"github.com/nikhea/malawi-e-commerce-store/internal/cart/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/idempotency"
)

// RegisterRoutes maps the cart HTTP surface. All routes live on the
// JWT-protected group (identity comes from the token); no admin surface.
// Route table only, no logic.
func RegisterRoutes(protected *gin.RouterGroup, svc public.Service, idem *idempotency.Store) {
	h := handler.New(svc)

	cart := protected.Group("/cart")
	cart.GET("", h.Get)
	// Static "bulk" segment is registered before the single add so it
	// always wins over any future path parameter.
	cart.POST("/items/bulk", idem.Middleware(), h.BulkAdd)
	cart.POST("/items", h.AddItem)
	cart.PATCH("/items/:id", h.SetQty)
	cart.DELETE("/items/:id", h.RemoveItem)
	cart.DELETE("", h.Clear)
}
