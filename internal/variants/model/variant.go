package model

import "time"

// Variant is the variants table row. Module-private.
type Variant struct {
	ID         string
	ProductID  string
	Name       string
	SKU        string
	PriceCents int64
	IsActive   bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
