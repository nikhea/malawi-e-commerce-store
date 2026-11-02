package dto

import (
	"github.com/nikhea/malawi-e-commerce-store/internal/cart/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/bulk"
)

// AddItemRequest is the POST /cart/items body. VariantID nil = base
// product price; set it for a specific SKU.
type AddItemRequest struct {
	ProductID string  `json:"product_id" binding:"required" example:"3fa85f64-5717-4562-b3fc-2c963f66afa6"`
	VariantID *string `json:"variant_id,omitempty"`
	Qty       int     `json:"qty" example:"2"`
}

// BulkAddItemsRequest is the POST /cart/items/bulk body. Identity comes
// from the JWT only.
type BulkAddItemsRequest struct {
	Atomic bool             `json:"atomic"`
	Items  []AddItemRequest `json:"items" binding:"required"`
}

// BulkAddResult is one line outcome. Declared concretely because swag cannot
// render generic instantiations.
type BulkAddResult struct {
	Index          int                 `json:"index" example:"0"`
	ID             string              `json:"id,omitempty"`
	Status         string              `json:"status" example:"ok"`
	Data           *public.Line        `json:"data,omitempty"`
	Error          *bulk.BulkItemError `json:"error,omitempty"`
	AlreadyExists  bool                `json:"alreadyExists,omitempty"`
	AlreadyRemoved bool                `json:"alreadyRemoved,omitempty"`
	WouldDelete    bool                `json:"wouldDelete,omitempty"`
	Blockers       []string            `json:"blockers,omitempty"`
}

// BulkAddCartResponse adds the full updated cart to the standard bulk
// envelope so the client never has to re-fetch after a batch add.
type BulkAddCartResponse struct {
	Results []BulkAddResult  `json:"results"`
	Summary bulk.BulkSummary `json:"summary"`
	Cart    public.Cart      `json:"cart"`
}

// NewBulkAddCartResponse projects the service results onto the documented
// response shape.
func NewBulkAddCartResponse(results []bulk.BulkResult[public.Line], summary bulk.BulkSummary, cart public.Cart) BulkAddCartResponse {
	out := make([]BulkAddResult, len(results))
	for i, r := range results {
		out[i] = BulkAddResult{
			Index: r.Index, ID: r.ID, Status: r.Status, Data: r.Data, Error: r.Error,
			AlreadyExists: r.AlreadyExists, AlreadyRemoved: r.AlreadyRemoved,
			WouldDelete: r.WouldDelete, Blockers: r.Blockers,
		}
	}
	return BulkAddCartResponse{Results: out, Summary: summary, Cart: cart}
}

// SetQtyRequest is the PATCH /cart/items/:id body. Qty 0 removes the line.
type SetQtyRequest struct {
	Qty int `json:"qty" example:"3"`
}

// ListCartQuery binds GET /cart paging and line filters. Totals in the
// response always cover the whole cart, not just the returned page.
type ListCartQuery struct {
	Q         string  `form:"q"`
	ProductID *string `form:"product_id"`
	VariantID *string `form:"variant_id"`
	Sort      string  `form:"sort"`
	Page      int     `form:"page"`
	Limit     int     `form:"limit"`
	Offset    int     `form:"offset"`
}
