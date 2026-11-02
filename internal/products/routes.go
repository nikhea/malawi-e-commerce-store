package products

import (
	"github.com/gin-gonic/gin"
	mediapublic "github.com/nikhea/malawi-e-commerce-store/internal/media/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/products/handler"
	"github.com/nikhea/malawi-e-commerce-store/internal/products/public"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/idempotency"
	"github.com/nikhea/malawi-e-commerce-store/pkg/middleware"
)

// RegisterRoutes maps the products HTTP surface. Storefront pattern:
// reads are OPEN, writes are admin-gated (same as categories).
func RegisterRoutes(open, protected *gin.RouterGroup, svc public.Service, media mediapublic.Service, idem *idempotency.Store) {
	h := handler.New(svc, media)

	prods := open.Group("/products")
	prods.GET("", h.List)
	prods.GET("/:id", h.GetByID)
	prods.GET("/slug/:slug", h.GetBySlug)

	admin := protected.Group("/admin", middleware.RequireRole(string(userspublic.RoleAdmin)))
	// Bulk writes are registered before the /:id routes so the static
	// "bulk" segments always win.
	admin.POST("/products/bulk", idem.Middleware(), h.BulkCreate)
	admin.PATCH("/products/bulk", idem.Middleware(), h.BulkUpdate)
	admin.POST("/products/bulk-delete", idem.Middleware(), h.BulkDelete)
	admin.POST("/products", h.Create)
	admin.PATCH("/products/:id", h.Update)
	admin.DELETE("/products/:id", h.Delete)
	admin.POST("/products/:id/image", h.UploadImage)
	admin.GET("/products/:id/images", h.ListImages)
	admin.POST("/products/:id/images", h.UploadImages)
	admin.DELETE("/products/:id/images/:imageId", h.DeleteImage)
	admin.PUT("/products/:id/images/:imageId/primary", h.SetPrimaryImage)
}
