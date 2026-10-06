// Package public is the orders module's external contract. Cart hands
// checkout here; payments confirms or cancels through here. Events
// (below) fan side effects out to future subscribers.
package public

import (
	"context"
	"time"
)

// Order statuses. pending → paid | cancelled. Terminal states never move.
const (
	StatusPending   = "pending"
	StatusPaid      = "paid"
	StatusCancelled = "cancelled"
)

// OrderLine is the legal record of one purchased line — snapshotted from
// the cart at checkout, immune to later catalog or cart changes.
type OrderLine struct {
	ProductID      string  `json:"product_id"`
	VariantID      *string `json:"variant_id,omitempty"`
	Name           string  `json:"name"`
	SKU            string  `json:"sku,omitempty"`
	UnitPriceCents int64   `json:"unit_price_cents"`
	Qty            int     `json:"qty"`
	LineTotalCents int64   `json:"line_total_cents"`
}

// Order is the checkout view: header + frozen lines.
type Order struct {
	ID            string      `json:"id"`
	UserID        string      `json:"user_id"`
	Status        string      `json:"status" example:"pending"`
	SubtotalCents int64       `json:"subtotal_cents"`
	Currency      string      `json:"currency" example:"MWK"`
	Lines         []OrderLine `json:"lines"`
	CreatedAt     time.Time   `json:"created_at"`
}

// Service is the orders capability surface. Owner-scoped methods take
// userID (from the JWT); MarkPaid/CancelByRef take the order ref for the
// payments module (sync path until payment webhooks land).
type Service interface {
	// Checkout freezes the cart into a pending order, reserves stock,
	// clears the cart, and notifies OrderCreated.
	Checkout(ctx context.Context, userID string) (Order, error)
	// GetByID returns one order, owner-checked.
	GetByID(ctx context.Context, userID, orderID string) (Order, error)
	// GetByRef returns one order WITHOUT owner check. Trusted internal
	// callers only (worker notifications) — never expose via HTTP.
	GetByRef(ctx context.Context, orderID string) (Order, error)
	// ListMine returns the caller's orders, newest first.
	ListMine(ctx context.Context, userID string) ([]Order, error)
	// Cancel releases stock and notifies OrderCancelled. Pending only.
	Cancel(ctx context.Context, userID, orderID string) (Order, error)
	// MarkPaid confirms stock and notifies OrderPaid. For payments.
	// Idempotent: paying twice succeeds (webhook retries demand it).
	MarkPaid(ctx context.Context, orderRef string) (Order, error)
	// CancelByRef releases stock for a ref (failed payments). Idempotent.
	CancelByRef(ctx context.Context, orderRef string) error
}
