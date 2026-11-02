package dto

import (
	"github.com/nikhea/malawi-e-commerce-store/internal/cart/public"
	bulkdto "github.com/nikhea/malawi-e-commerce-store/pkg/bulk/dto"
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

// BulkAddResult is one line outcome. A named type keeps swag happy.
type BulkAddResult bulkdto.BulkResult[public.Line]

// BulkAddSummary mirrors bulkdto.BulkSummary. It is duplicated here because
// swag cannot resolve the shared package alias through a struct field.
type BulkAddSummary struct {
	Total     int `json:"total" example:"2"`
	Succeeded int `json:"succeeded" example:"1"`
	Failed    int `json:"failed" example:"1"`
}

// BulkAddCartResponse adds the full updated cart to the standard bulk
// envelope so the client never has to re-fetch after a batch add.
type BulkAddCartResponse struct {
	Results []BulkAddResult `json:"results"`
	Summary BulkAddSummary  `json:"summary"`
	Cart    public.Cart     `json:"cart"`
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
