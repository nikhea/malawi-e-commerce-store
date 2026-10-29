package model

import "time"

// Cart is the carts table row. Module-private.
type Cart struct {
	ID        string
	UserID    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Item is a cart_items row, price already snapshotted at add-time.
type Item struct {
	ID             string
	CartID         string
	ProductID      string
	VariantID      *string
	Name           string
	SKU            string
	UnitPriceCents int64
	Currency       string
	Qty            int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ListFilter scopes a paged cart-items read. Limit <= 0 means "no limit"
// (internal callers need the whole cart for totals/checkout).
type ListFilter struct {
	Query     string
	ProductID *string
	VariantID *string
	Sort      string
	Limit     int
	Offset    int
}
