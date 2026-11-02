package dto

import (
	"github.com/nikhea/malawi-e-commerce-store/internal/variants/public"
	bulkdto "github.com/nikhea/malawi-e-commerce-store/pkg/bulk/dto"
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

// Named response types keep swag happy: it cannot parse a generic
// instantiation inside an annotation, but resolves a named alias.
type BulkCreateVariantsResponse bulkdto.BulkResponse[public.Variant]
type BulkUpdateVariantsResponse bulkdto.BulkResponse[public.Variant]
