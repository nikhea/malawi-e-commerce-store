package wishlist

import (
	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/internal/wishlist/handler"
	"github.com/nikhea/malawi-e-commerce-store/internal/wishlist/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/idempotency"
)

// RegisterRoutes maps the wishlist HTTP surface. JWT-only; identity from
// the token. Bulk static segments are registered before the id route.
func RegisterRoutes(protected *gin.RouterGroup, svc public.Service, idem *idempotency.Store) {
	h := handler.New(svc)

	wishlist := protected.Group("/wishlist")
	wishlist.GET("", h.ListMine)
	wishlist.POST("/bulk", idem.Middleware(), h.BulkAdd)
	wishlist.POST("/bulk-delete", idem.Middleware(), h.BulkRemove)
	wishlist.POST("", h.Add)
	wishlist.DELETE("/:id", h.Remove)
}
