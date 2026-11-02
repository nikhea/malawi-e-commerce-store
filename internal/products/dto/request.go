package dto

import (
	"github.com/nikhea/malawi-e-commerce-store/internal/products/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/bulk"
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

// BulkProductResult is one product outcome. Declared concretely because swag
// cannot render generic instantiations.
type BulkProductResult struct {
	Index          int                 `json:"index" example:"0"`
	ID             string              `json:"id,omitempty"`
	Status         string              `json:"status" example:"ok"`
	Data           *public.Product     `json:"data,omitempty"`
	Error          *bulk.BulkItemError `json:"error,omitempty"`
	AlreadyExists  bool                `json:"alreadyExists,omitempty"`
	AlreadyRemoved bool                `json:"alreadyRemoved,omitempty"`
	WouldDelete    bool                `json:"wouldDelete,omitempty"`
	Blockers       []string            `json:"blockers,omitempty"`
}

// BulkDeleteResult is the delete-style outcome (no entity payload).
type BulkDeleteResult struct {
	Index          int                 `json:"index" example:"0"`
	ID             string              `json:"id,omitempty"`
	Status         string              `json:"status" example:"ok"`
	Error          *bulk.BulkItemError `json:"error,omitempty"`
	WouldDelete    bool                `json:"wouldDelete,omitempty"`
	Blockers       []string            `json:"blockers,omitempty"`
	AlreadyRemoved bool                `json:"alreadyRemoved,omitempty"`
}

// Documented bulk response shapes.
type BulkCreateProductsResponse struct {
	Results []BulkProductResult `json:"results"`
	Summary bulk.BulkSummary    `json:"summary"`
}

type BulkUpdateProductsResponse struct {
	Results []BulkProductResult `json:"results"`
	Summary bulk.BulkSummary    `json:"summary"`
}

type BulkDeleteProductsResponse struct {
	Results []BulkDeleteResult `json:"results"`
	Summary bulk.BulkSummary   `json:"summary"`
}

// NewBulkProductResponse projects product service results onto the
// documented shape.
func NewBulkProductResponse(results []bulk.BulkResult[public.Product], summary bulk.BulkSummary) BulkCreateProductsResponse {
	out := make([]BulkProductResult, len(results))
	for i, r := range results {
		out[i] = BulkProductResult{
			Index: r.Index, ID: r.ID, Status: r.Status, Data: r.Data, Error: r.Error,
			AlreadyExists: r.AlreadyExists, AlreadyRemoved: r.AlreadyRemoved,
			WouldDelete: r.WouldDelete, Blockers: r.Blockers,
		}
	}
	return BulkCreateProductsResponse{Results: out, Summary: summary}
}

// NewBulkDeleteResponse projects delete results onto the documented shape.
func NewBulkDeleteResponse(results []bulk.BulkResult[bulk.Empty], summary bulk.BulkSummary) BulkDeleteProductsResponse {
	out := make([]BulkDeleteResult, len(results))
	for i, r := range results {
		out[i] = BulkDeleteResult{
			Index: r.Index, ID: r.ID, Status: r.Status, Error: r.Error,
			WouldDelete: r.WouldDelete, Blockers: r.Blockers,
			AlreadyRemoved: r.AlreadyRemoved,
		}
	}
	return BulkDeleteProductsResponse{Results: out, Summary: summary}
}

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
