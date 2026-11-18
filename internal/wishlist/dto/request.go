package dto

import (
	"github.com/nikhea/malawi-e-commerce-store/internal/wishlist/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/bulk"
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

// BulkWishlistResult is one heart outcome. Declared concretely because swag
// cannot render generic instantiations.
type BulkWishlistResult struct {
	Index          int                 `json:"index" example:"0"`
	ID             string              `json:"id,omitempty"`
	Status         string              `json:"status" example:"ok"`
	Data           *public.Item        `json:"data,omitempty"`
	Error          *bulk.BulkItemError `json:"error,omitempty"`
	AlreadyExists  bool                `json:"alreadyExists,omitempty"`
	AlreadyRemoved bool                `json:"alreadyRemoved,omitempty"`
	WouldDelete    bool                `json:"wouldDelete,omitempty"`
	Blockers       []string            `json:"blockers,omitempty"`
}

// BulkWishlistDeleteResult is the unheart outcome (no entity payload).
type BulkWishlistDeleteResult struct {
	Index          int                 `json:"index" example:"0"`
	ID             string              `json:"id,omitempty"`
	Status         string              `json:"status" example:"ok"`
	Error          *bulk.BulkItemError `json:"error,omitempty"`
	AlreadyRemoved bool                `json:"alreadyRemoved,omitempty"`
	WouldDelete    bool                `json:"wouldDelete,omitempty"`
	Blockers       []string            `json:"blockers,omitempty"`
}

// Documented bulk response shapes.
type BulkAddWishlistResponse struct {
	Results []BulkWishlistResult `json:"results"`
	Summary bulk.BulkSummary     `json:"summary"`
}

type BulkRemoveWishlistResponse struct {
	Results []BulkWishlistDeleteResult `json:"results"`
	Summary bulk.BulkSummary           `json:"summary"`
}

// NewBulkAddWishlistResponse projects heart results onto the documented
// shape.
func NewBulkAddWishlistResponse(results []bulk.BulkResult[public.Item], summary bulk.BulkSummary) BulkAddWishlistResponse {
	out := make([]BulkWishlistResult, len(results))
	for i, r := range results {
		out[i] = BulkWishlistResult{
			Index: r.Index, ID: r.ID, Status: r.Status, Data: r.Data, Error: r.Error,
			AlreadyExists: r.AlreadyExists, AlreadyRemoved: r.AlreadyRemoved,
			WouldDelete: r.WouldDelete, Blockers: r.Blockers,
		}
	}
	return BulkAddWishlistResponse{Results: out, Summary: summary}
}

// NewBulkRemoveWishlistResponse projects unheart results onto the
// documented shape.
func NewBulkRemoveWishlistResponse(results []bulk.BulkResult[bulk.Empty], summary bulk.BulkSummary) BulkRemoveWishlistResponse {
	out := make([]BulkWishlistDeleteResult, len(results))
	for i, r := range results {
		out[i] = BulkWishlistDeleteResult{
			Index: r.Index, ID: r.ID, Status: r.Status, Error: r.Error,
			AlreadyRemoved: r.AlreadyRemoved, WouldDelete: r.WouldDelete,
			Blockers: r.Blockers,
		}
	}
	return BulkRemoveWishlistResponse{Results: out, Summary: summary}
}

// ListWishlistQuery binds GET /wishlist paging and catalog filters.
type ListWishlistQuery struct {
	Q          string  `form:"q"`
	CategoryID *string `form:"category_id"`
	Sort       string  `form:"sort"`
	Page       int     `form:"page"`
	Limit      int     `form:"limit"`
	Offset     int     `form:"offset"`
}
