package dto

// CreateProductRequest is the POST /admin/products body. Price is USD
// cents: 12500 = $125.00.
type CreateProductRequest struct {
	CategoryID  *string `json:"category_id,omitempty"`
	Name        string  `json:"name" binding:"required" example:"Sunseed Cooking Oil 2L"`
	Slug        string  `json:"slug" example:"sunseed-cooking-oil-2l"`
	Description string  `json:"description" example:"Pure sunflower oil"`
	PriceCents  int64   `json:"price_cents" example:"12500"`
	Currency    string  `json:"currency" example:"USD"`
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

// ListProductsQuery binds the storefront catalog filters. Dates accept
// RFC3339 or YYYY-MM-DD; page wins over offset when both are sent.
type ListProductsQuery struct {
	Q             string   `form:"q"`
	CategoryID    *string  `form:"category_id"`
	CategorySlug  *string  `form:"category_slug"`
	MinPriceCents *int64   `form:"min_price_cents"`
	MaxPriceCents *int64   `form:"max_price_cents"`
	InStock       *bool    `form:"in_stock"`
	Size          string   `form:"size"`
	MinRating     *float64 `form:"min_rating"`
	CreatedAfter  string   `form:"created_after"`
	CreatedBefore string   `form:"created_before"`
	ActiveOnly    *bool    `form:"active_only"`
	Sort          string   `form:"sort"`
	Page          int      `form:"page"`
	Limit         int      `form:"limit"`
	Offset        int      `form:"offset"`
}
