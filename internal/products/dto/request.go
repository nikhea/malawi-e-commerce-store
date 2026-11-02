package dto

import (
	"github.com/nikhea/malawi-e-commerce-store/internal/products/public"
	bulkdto "github.com/nikhea/malawi-e-commerce-store/pkg/bulk/dto"
)

// CreateProductRequest is the POST /admin/products body. Price is USD
// cents: 12500 = $125.00.
type CreateProductRequest struct {
	CategoryID  *string `json:"category_id,omitempty"`
	Name        string  `json:"name" binding:"required" example:"Sunseed Cooking Oil 2L"`
	Slug        string  `json:"slug" example:"sunseed-cooking-oil-2l"`
	Description string  `json:"description" example:"Pure sunflower oil"`
	PriceCents  int64   `json:"price_cents" example:"12500"`
	Currency    string  `json:"currency" example:"USD"`
	ImageURL    string  `json:"image_url,omitempty"`
}

// UpdateProductRequest is the PATCH /admin/products/:id body.
// Pointers distinguish "absent" from "zero".
type UpdateProductRequest struct {
	CategoryID    *string `json:"category_id,omitempty"`
	Name          *string `json:"name,omitempty"`
	Description   *string `json:"description,omitempty"`
	PriceCents    *int64  `json:"price_cents,omitempty"`
	Currency      *string `json:"currency,omitempty"`
	IsActive      *bool   `json:"is_active,omitempty"`
	ClearCategory bool    `json:"clear_category,omitempty"`
}

// BulkCreateProductsRequest is the POST /admin/products/bulk body.
type BulkCreateProductsRequest struct {
	Atomic bool                   `json:"atomic"`
	DryRun bool                   `json:"dryRun"`
	Items  []CreateProductRequest `json:"items" binding:"required"`
}

// BulkUpdateProductItem pairs one product id with the fields to patch.
type BulkUpdateProductItem struct {
	ID    string               `json:"id" binding:"required" example:"3fa85f64-5717-4562-b3fc-2c963f66afa6"`
	Patch UpdateProductRequest `json:"patch" binding:"required"`
}

// BulkUpdateProductsRequest is the PATCH /admin/products/bulk body.
type BulkUpdateProductsRequest struct {
	Atomic  bool                    `json:"atomic"`
	DryRun  bool                    `json:"dryRun"`
	Updates []BulkUpdateProductItem `json:"updates" binding:"required"`
}

// BulkDeleteProductsRequest is the POST /admin/products/bulk-delete body.
// Delete is a POST so the id list can travel in JSON.
type BulkDeleteProductsRequest struct {
	IDs    []string `json:"ids" binding:"required"`
	DryRun bool     `json:"dryRun"`
	Atomic bool     `json:"atomic"`
}

// Named response types keep swag happy: it cannot parse a generic
// instantiation inside an annotation, but resolves a named alias.
type BulkCreateProductsResponse bulkdto.BulkResponse[public.Product]
type BulkUpdateProductsResponse bulkdto.BulkResponse[public.Product]
type BulkDeleteProductsResponse bulkdto.BulkResponse[bulkdto.Empty]

// ListProductsQuery binds the storefront catalog filters. Dates accept
// RFC3339 or YYYY-MM-DD; page wins over offset when both are sent.
type ListProductsQuery struct {
	// Q is full-text search across name, description and slug. Word
	// prefixes match; substring search is the fallback when FTS misses.
	Q             string   `form:"q"`
	CategoryID    *string  `form:"category_id"`
	CategorySlug  *string  `form:"category_slug"`
	MinPriceCents *int64   `form:"min_price_cents"`
	MaxPriceCents *int64   `form:"max_price_cents"`
	InStock       *bool    `form:"in_stock"`
	Size          string   `form:"size"`
	MinRating     *float64 `form:"min_rating"`
	CreatedAfter  string   `form:"created_after"`
	CreatedBefore string   `form:"created_before"`
	ActiveOnly    *bool    `form:"active_only"`
	Sort          string   `form:"sort"`
	Page          int      `form:"page"`
	Limit         int      `form:"limit"`
	Offset        int      `form:"offset"`
}
