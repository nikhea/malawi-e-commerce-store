package dto

import (
	"github.com/nikhea/malawi-e-commerce-store/internal/variants/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/bulk"
)

// CreateVariantRequest is the POST /products/:id/variants body.
type CreateVariantRequest struct {
	Name       string `json:"name" binding:"required" example:"500ml"`
	SKU        string `json:"sku" binding:"required" example:"OIL-500-001"`
	PriceCents int64  `json:"price_cents" example:"2500"`
}

// UpdateVariantRequest is the PATCH /admin/variants/:id body.
// Pointers distinguish "absent" from "zero".
type UpdateVariantRequest struct {
	Name       *string `json:"name,omitempty"`
	PriceCents *int64  `json:"price_cents,omitempty"`
	IsActive   *bool   `json:"is_active,omitempty"`
}

// BulkCreateVariantsRequest is the POST /products/:id/variants/bulk body.
type BulkCreateVariantsRequest struct {
	Atomic bool                   `json:"atomic"`
	DryRun bool                   `json:"dryRun"`
	Items  []CreateVariantRequest `json:"items" binding:"required"`
}

// BulkUpdateVariantItem pairs a variant id with its partial patch.
type BulkUpdateVariantItem struct {
	ID    string               `json:"id" binding:"required" example:"3fa85f64-5717-4562-b3fc-2c963f66afa6"`
	Patch UpdateVariantRequest `json:"patch" binding:"required"`
}

// BulkUpdateVariantsRequest is the PATCH /admin/variants/bulk body.
type BulkUpdateVariantsRequest struct {
	Atomic  bool                    `json:"atomic"`
	DryRun  bool                    `json:"dryRun"`
	Updates []BulkUpdateVariantItem `json:"updates" binding:"required"`
}

// BulkVariantResult is one variant outcome. Declared concretely because swag
// cannot render generic instantiations.
type BulkVariantResult struct {
	Index          int                 `json:"index" example:"0"`
	ID             string              `json:"id,omitempty"`
	Status         string              `json:"status" example:"ok"`
	Data           *public.Variant     `json:"data,omitempty"`
	Error          *bulk.BulkItemError `json:"error,omitempty"`
	AlreadyExists  bool                `json:"alreadyExists,omitempty"`
	AlreadyRemoved bool                `json:"alreadyRemoved,omitempty"`
	WouldDelete    bool                `json:"wouldDelete,omitempty"`
	Blockers       []string            `json:"blockers,omitempty"`
}

// Documented bulk response shapes.
type BulkCreateVariantsResponse struct {
	Results []BulkVariantResult `json:"results"`
	Summary bulk.BulkSummary    `json:"summary"`
}

type BulkUpdateVariantsResponse struct {
	Results []BulkVariantResult `json:"results"`
	Summary bulk.BulkSummary    `json:"summary"`
}

// NewBulkVariantResponse projects variant service results onto the
// documented shape.
func NewBulkVariantResponse(results []bulk.BulkResult[public.Variant], summary bulk.BulkSummary) BulkCreateVariantsResponse {
	out := make([]BulkVariantResult, len(results))
	for i, r := range results {
		out[i] = BulkVariantResult{
			Index: r.Index, ID: r.ID, Status: r.Status, Data: r.Data, Error: r.Error,
			AlreadyExists: r.AlreadyExists, AlreadyRemoved: r.AlreadyRemoved,
			WouldDelete: r.WouldDelete, Blockers: r.Blockers,
		}
	}
	return BulkCreateVariantsResponse{Results: out, Summary: summary}
}
