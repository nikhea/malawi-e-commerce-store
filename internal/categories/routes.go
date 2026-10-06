package categories

import (
	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/internal/categories/handler"
	"github.com/nikhea/malawi-e-commerce-store/internal/categories/public"
	mediapublic "github.com/nikhea/malawi-e-commerce-store/internal/media/public"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/middleware"
)

// RegisterRoutes maps the categories HTTP surface. Storefront pattern:
// reads are OPEN (browsing needs no login), writes are admin-gated.
// open carries no auth middleware; protected already runs JWT, and the
// admin subgroup adds RequireRole. The role value comes from the users
// contract (importing another module's public/ is the allowed seam).
func RegisterRoutes(open, protected *gin.RouterGroup, svc public.Service, media mediapublic.Service) {
	h := handler.New(svc, media)

	cats := open.Group("/categories")
	cats.GET("", h.List)
	cats.GET("/:id", h.GetByID)
	cats.GET("/slug/:slug", h.GetBySlug)

	admin := protected.Group("/admin", middleware.RequireRole(string(userspublic.RoleAdmin)))
	admin.POST("/categories", h.Create)
	admin.PATCH("/categories/:id", h.Update)
	admin.DELETE("/categories/:id", h.Delete)
	admin.POST("/categories/:id/image", h.UploadImage)
}
