// Package public is the inventory module's external contract. Orders
// reserves stock through here at checkout; payments confirms or releases
// via events. Absolute quantities only — never deltas without a reason.
package public

import (
	"context"
	"time"
)

// Stock is the availability view. Available = OnHand - Reserved.
type Stock struct {
	ProductID string  `json:"product_id"`
	VariantID *string `json:"variant_id,omitempty"`
	OnHand    int     `json:"on_hand" example:"50"`
	Reserved  int     `json:"reserved" example:"3"`
	Available int     `json:"available" example:"47"`
}

// ReserveLine is one SKU slice of a reservation.
type ReserveLine struct {
	ProductID string
	VariantID *string
	Qty       int
}

// Service is the inventory capability surface.
type Service interface {
	// SetStock upserts the on-hand quantity (admin restocking).
	SetStock(ctx context.Context, productID string, variantID *string, qty int) (Stock, error)
	// GetStock reads availability. Unknown SKU = zero stock, NOT an
	// error (untracked items simply can't be reserved).
	GetStock(ctx context.Context, productID string, variantID *string) (Stock, error)
	// Reserve holds qty for orderRef for ttl. Fails with CONFLICT when
	// any line lacks availability. Atomic: all lines or none.
	Reserve(ctx context.Context, orderRef string, lines []ReserveLine, ttl time.Duration) error
	// Release frees an order's active holds (cancel/expire path).
	ReleaseByOrder(ctx context.Context, orderRef string) error
	// ConfirmByOrder converts holds into sales (payment path).
	ConfirmByOrder(ctx context.Context, orderRef string) error
	// ReleaseExpired frees all holds past their TTL and returns the
	// affected order refs (worker expiry sweep; orders cancels them).
	ReleaseExpired(ctx context.Context) ([]string, error)
}
