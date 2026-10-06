package dto

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
