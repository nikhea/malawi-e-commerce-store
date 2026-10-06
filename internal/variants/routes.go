package variants

import (
	"github.com/gin-gonic/gin"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/variants/handler"
	"github.com/nikhea/malawi-e-commerce-store/internal/variants/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/middleware"
)

// RegisterRoutes maps the variants HTTP surface. Reads nest under the
// product path on the OPEN group; writes are admin-gated. Same method +
// path never repeats across groups (GET vs POST on the nested path).
func RegisterRoutes(open, protected *gin.RouterGroup, svc public.Service) {
	h := handler.New(svc)

	nested := open.Group("/products/:id/variants")
	nested.GET("", h.ListByProduct)

	adminNested := protected.Group("/products/:id/variants", middleware.RequireRole(string(userspublic.RoleAdmin)))
	adminNested.POST("", h.Create)

	admin := protected.Group("/admin", middleware.RequireRole(string(userspublic.RoleAdmin)))
	admin.PATCH("/variants/:id", h.Update)
	admin.DELETE("/variants/:id", h.Delete)
}
