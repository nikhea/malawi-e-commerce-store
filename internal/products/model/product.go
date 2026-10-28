package model

import "time"

// ListFilter is the repository-level catalog query. Values are already
// normalized and validated by the service layer.
type ListFilter struct {
	Query         string
	CategoryID    *string
	CategorySlug  *string
	MinPriceCents *int64
	MaxPriceCents *int64
	InStock       *bool
	Size          string
	MinRating     *float64
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	ActiveOnly    bool
	Sort          string
	Limit         int
	Offset        int
}

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
