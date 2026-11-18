package dto

import (
	"github.com/nikhea/malawi-e-commerce-store/internal/inventory/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/bulk"
)

// SetStockRequest is the POST /admin/inventory/stock body.
type SetStockRequest struct {
	ProductID string  `json:"product_id" binding:"required"`
	VariantID *string `json:"variant_id,omitempty"`
	Qty       int     `json:"qty" example:"50"`
}

// BulkSetStockItem is one line of a batch restock. Stock is keyed by
// product + variant, so both ids are required to address a SKU.
type BulkSetStockItem struct {
	ProductID string  `json:"product_id" binding:"required" example:"3fa85f64-5717-4562-b3fc-2c963f66afa6"`
	VariantID *string `json:"variant_id,omitempty"`
	Qty       int     `json:"qty" example:"120"`
}

// BulkSetStockRequest is the POST /admin/inventory/stock/bulk body.
type BulkSetStockRequest struct {
	Atomic bool               `json:"atomic"`
	Items  []BulkSetStockItem `json:"items" binding:"required"`
}

// BulkSetStockResult is one stock line outcome. Declared concretely because
// swag cannot render generic instantiations.
type BulkSetStockResult struct {
	Index          int                 `json:"index" example:"0"`
	ID             string              `json:"id,omitempty"`
	Status         string              `json:"status" example:"ok"`
	Data           *public.Stock       `json:"data,omitempty"`
	Error          *bulk.BulkItemError `json:"error,omitempty"`
	AlreadyExists  bool                `json:"alreadyExists,omitempty"`
	AlreadyRemoved bool                `json:"alreadyRemoved,omitempty"`
	WouldDelete    bool                `json:"wouldDelete,omitempty"`
	Blockers       []string            `json:"blockers,omitempty"`
}

// BulkSetStockResponse is the documented bulk restock response.
type BulkSetStockResponse struct {
	Results []BulkSetStockResult `json:"results"`
	Summary bulk.BulkSummary     `json:"summary"`
}

// NewBulkSetStockResponse projects the service results onto the documented
// response shape.
func NewBulkSetStockResponse(results []bulk.BulkResult[public.Stock], summary bulk.BulkSummary) BulkSetStockResponse {
	out := make([]BulkSetStockResult, len(results))
	for i, r := range results {
		out[i] = BulkSetStockResult{
			Index: r.Index, ID: r.ID, Status: r.Status, Data: r.Data, Error: r.Error,
			AlreadyExists: r.AlreadyExists, AlreadyRemoved: r.AlreadyRemoved,
			WouldDelete: r.WouldDelete, Blockers: r.Blockers,
		}
	}
	return BulkSetStockResponse{Results: out, Summary: summary}
}

// BulkStockResponse is the variantIds read: availability rows plus the ids
// that did not resolve to a variant.
type BulkStockResponse struct {
	Stock    []public.Stock `json:"stock"`
	NotFound []string       `json:"notFound"`
}

// ReserveLineRequest is one SKU slice of a reservation.
type ReserveLineRequest struct {
	ProductID string  `json:"product_id" binding:"required"`
	VariantID *string `json:"variant_id,omitempty"`
	Qty       int     `json:"qty" binding:"required" example:"2"`
}

// ReserveRequest is the POST /admin/inventory/reserve body.
type ReserveRequest struct {
	OrderRef   string               `json:"order_ref" binding:"required" example:"order_123"`
	Lines      []ReserveLineRequest `json:"lines" binding:"required"`
	TTLSeconds int64                `json:"ttl_seconds" example:"900"`
}

// OrderRefRequest is the release/confirm body.
type OrderRefRequest struct {
	OrderRef string `json:"order_ref" binding:"required" example:"order_123"`
}
