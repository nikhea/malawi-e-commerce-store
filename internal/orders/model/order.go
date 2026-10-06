package model

import "time"

// Order is the orders table row. Module-private.
type Order struct {
	ID            string
	UserID        string
	Status        string
	SubtotalCents int64
	Currency      string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Item is an order_items row. Module-private.
type Item struct {
	ID             string
	OrderID        string
	ProductID      string
	VariantID      *string
	Name           string
	SKU            string
	UnitPriceCents int64
	Qty            int
}
