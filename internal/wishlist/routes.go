package wishlist

import (
	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/internal/wishlist/handler"
	"github.com/nikhea/malawi-e-commerce-store/internal/wishlist/public"
)

// RegisterRoutes maps the wishlist HTTP surface. JWT-only; identity from
// the token. Route table only, no logic.
func RegisterRoutes(protected *gin.RouterGroup, svc public.Service) {
	h := handler.New(svc)

	wishlist := protected.Group("/wishlist")
	wishlist.GET("", h.ListMine)
	wishlist.POST("", h.Add)
	wishlist.DELETE("/:id", h.Remove)
}
