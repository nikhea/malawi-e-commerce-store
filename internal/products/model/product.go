package model

import "time"

// Product is the products table row. Module-private.
type Product struct {
	ID            string
	CategoryID    *string
	CategoryName  string // LEFT JOIN, read-only
	Name          string
	Slug          string
	Description   string
	PriceCents    int64
	Currency      string
	ImageURL      string
	ImagePublicID string
	IsActive      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
