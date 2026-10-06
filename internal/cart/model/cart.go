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
