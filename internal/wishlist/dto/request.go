package dto

import (
	"github.com/nikhea/malawi-e-commerce-store/internal/wishlist/public"
	bulkdto "github.com/nikhea/malawi-e-commerce-store/pkg/bulk/dto"
)

// AddWishlistRequest is the POST /wishlist body.
type AddWishlistRequest struct {
	ProductID string `json:"product_id" binding:"required" example:"3fa85f64-5717-4562-b3fc-2c963f66afa6"`
}

// BulkAddWishlistRequest is the POST /wishlist/bulk body. The field is
// camelCase because the spec calls it out explicitly; per-item payloads
// reuse the existing snake_case DTOs elsewhere.
type BulkAddWishlistRequest struct {
	ProductIDs []string `json:"productIds" binding:"required"`
	Atomic     bool     `json:"atomic"`
}

// BulkRemoveWishlistRequest is the POST /wishlist/bulk-delete body. The
// identifiers are product ids, matching DELETE /wishlist/{id}.
type BulkRemoveWishlistRequest struct {
	IDs    []string `json:"ids" binding:"required"`
	DryRun bool     `json:"dryRun"`
	Atomic bool     `json:"atomic"`
}

// Named response types keep swag happy.
type BulkAddWishlistResponse bulkdto.BulkResponse[public.Item]
type BulkRemoveWishlistResponse bulkdto.BulkResponse[bulkdto.Empty]

// ListWishlistQuery binds GET /wishlist paging and catalog filters.
type ListWishlistQuery struct {
	Q          string  `form:"q"`
	CategoryID *string `form:"category_id"`
	Sort       string  `form:"sort"`
	Page       int     `form:"page"`
	Limit      int     `form:"limit"`
	Offset     int     `form:"offset"`
}
