// Package public is the cart module's external contract. Orders reads
// this at checkout (lines + snapshot prices); the cart never exposes
// other modules' internals back.
package public

import (
	"context"
	"time"
)

// Line is one cart row with its price snapshot. Variant fields empty =
// simple product (priced at the base price).
type Line struct {
	ID             string  `json:"id"`
	ProductID      string  `json:"product_id"`
	VariantID      *string `json:"variant_id,omitempty"`
	Name           string  `json:"name" example:"Sunseed Cooking Oil 2L"`
	SKU            string  `json:"sku,omitempty" example:"OIL-500-001"`
	UnitPriceCents int64   `json:"unit_price_cents" example:"12500"`
	Currency       string  `json:"currency" example:"MWK"`
	Qty            int     `json:"qty" example:"2"`
	LineTotalCents int64   `json:"line_total_cents" example:"25000"`
}

// Cart is the caller's active cart with computed totals.
type Cart struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	Items         []Line    `json:"items"`
	SubtotalCents int64     `json:"subtotal_cents"`
	Currency      string    `json:"currency" example:"MWK"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// AddItemInput carries a new line. VariantID nil = base product price.
type AddItemInput struct {
	UserID    string
	ProductID string
	VariantID *string
	Qty       int
}

// Service is the cart capability surface. All identity comes from the
// JWT (UserID args) — handlers never trust client-sent user ids.
type Service interface {
	// Get returns the active cart, creating an empty one on first use.
	Get(ctx context.Context, userID string) (Cart, error)
	// AddItem validates against the catalog, snapshots the price, and
	// merges qty into an existing line for the same product/variant.
	AddItem(ctx context.Context, in AddItemInput) (Cart, error)
	// SetQty changes a line's quantity; qty 0 removes the line.
	SetQty(ctx context.Context, userID, itemID string, qty int) (Cart, error)
	// RemoveItem drops one line. Unknown id = NOT_FOUND.
	RemoveItem(ctx context.Context, userID, itemID string) (Cart, error)
	// Clear empties the cart (used after successful checkout).
	Clear(ctx context.Context, userID string) error
}
