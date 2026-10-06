package dto

// SetStockRequest is the POST /admin/inventory/stock body.
type SetStockRequest struct {
	ProductID string  `json:"product_id" binding:"required"`
	VariantID *string `json:"variant_id,omitempty"`
	Qty       int     `json:"qty" example:"50"`
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
