package dto

// AddItemRequest is the POST /cart/items body. VariantID nil = base
// product price; set it for a specific SKU.
type AddItemRequest struct {
	ProductID string  `json:"product_id" binding:"required" example:"3fa85f64-5717-4562-b3fc-2c963f66afa6"`
	VariantID *string `json:"variant_id,omitempty"`
	Qty       int     `json:"qty" example:"2"`
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
