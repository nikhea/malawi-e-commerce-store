package model

import "time"

// Payment is the payments table row. Module-private.
type Payment struct {
	ID               string
	OrderID          string
	StripeIntentID   string
	ClientSecret     string
	AmountCents      int64
	Currency         string
	OrderAmountCents int64
	Status           string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
