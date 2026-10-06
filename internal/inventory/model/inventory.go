package model

import "time"

// Stock is the inventory table row. Module-private.
type Stock struct {
	ID        string
	ProductID string
	VariantID *string
	OnHand    int
	Reserved  int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Reservation is a reservations ledger row. Module-private.
type Reservation struct {
	ID        string
	ProductID string
	VariantID *string
	Qty       int
	OrderRef  string
	Status    string
	ExpiresAt time.Time
	CreatedAt time.Time
}
