package reviews

import (
	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/internal/reviews/handler"
	"github.com/nikhea/malawi-e-commerce-store/internal/reviews/public"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/middleware"
)

// RegisterRoutes maps the reviews HTTP surface. Reads nest under the
// product path on the OPEN group; the caller's own writes live on the
// protected group; moderation is admin-gated. Route table only.
func RegisterRoutes(open, protected *gin.RouterGroup, svc public.Service) {
	h := handler.New(svc)

	nested := open.Group("/products/:id/reviews")
	nested.GET("", h.Summary)

	ownNested := protected.Group("/products/:id/reviews")
	ownNested.POST("", h.Create)

	own := protected.Group("/reviews")
	own.PATCH("/:id", h.UpdateMine)
	own.DELETE("/:id", h.DeleteMine)

	admin := protected.Group("/admin", middleware.RequireRole(string(userspublic.RoleAdmin)))
	admin.DELETE("/reviews/:id", h.DeleteAny)
}
