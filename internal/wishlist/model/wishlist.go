package model

import "time"

// Item is a wishlist_items row. Module-private: the product view comes
// from products/public at read time, never stored here.
type Item struct {
	ID        string
	UserID    string
	ProductID string
	CreatedAt time.Time
}

// ListFilter scopes the caller's paged wishlist. Catalog fields (name,
// slug, price, category) are joined from products at read time.
type ListFilter struct {
	Query      string
	CategoryID *string
	Sort       string
	Limit      int
	Offset     int
}
