// Package public is the payments module's external contract. Orders
// moves money states through here; the Stripe webhook lands here.
// Events fan payment outcomes out to future subscribers (worker mail).
package public

import (
	"context"
	"time"
)

// Payment statuses. pending → succeeded | failed | cancelled.
const (
	StatusPending   = "pending"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

// Domain events emitted by this module. Payload is always Payment.
const (
	PaymentSucceeded = "payments.succeeded"
	PaymentFailed    = "payments.failed"
)

// Payment is the charge view. AmountCents is the CHARGED amount (USD
// cents); OrderAmountCents preserves the source MWK tambala.
type Payment struct {
	ID               string    `json:"id"`
	OrderID          string    `json:"order_id"`
	StripeIntentID   string    `json:"stripe_intent_id"`
	ClientSecret     string    `json:"client_secret"`
	AmountCents      int64     `json:"amount_cents" example:"735"`
	Currency         string    `json:"currency" example:"USD"`
	OrderAmountCents int64     `json:"order_amount_cents" example:"12500"`
	Status           string    `json:"status" example:"pending"`
	CreatedAt        time.Time `json:"created_at"`
}

// Service is the payments capability surface.
type Service interface {
	// CreateIntent validates ownership, converts the order total, and
	// returns a Stripe client secret. Retried calls reuse the pending
	// intent instead of double-charging.
	CreateIntent(ctx context.Context, userID, orderID string) (Payment, error)
	// HandleWebhook verifies the Stripe signature and settles the order.
	// Unknown event kinds are ignored (nil). Invalid signatures error —
	// the handler answers 4xx so Stripe does NOT retry forgeries.
	HandleWebhook(ctx context.Context, payload []byte, sigHeader string) error
}
