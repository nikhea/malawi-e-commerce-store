package dto

// CreateProductRequest is the POST /admin/products body. Price is minor
// units (tambala): 12500 = K125.00.
type CreateProductRequest struct {
	CategoryID  *string `json:"category_id,omitempty"`
	Name        string  `json:"name" binding:"required" example:"Sunseed Cooking Oil 2L"`
	Slug        string  `json:"slug" example:"sunseed-cooking-oil-2l"`
	Description string  `json:"description" example:"Pure sunflower oil"`
	PriceCents  int64   `json:"price_cents" example:"12500"`
	Currency    string  `json:"currency" example:"MWK"`
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

// ListProductsQuery binds ?category_id=&active_only=&limit=&offset=.
type ListProductsQuery struct {
	CategoryID *string `form:"category_id"`
	ActiveOnly *bool   `form:"active_only"`
	Limit      int     `form:"limit"`
	Offset     int     `form:"offset"`
}
