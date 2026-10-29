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

// ListFilter scopes the caller's paged order history.
type ListFilter struct {
	Status        string
	Currency      string
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	MinTotalCents *int64
	MaxTotalCents *int64
	Sort          string
	Limit         int
	Offset        int
}
