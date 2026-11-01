package model

import "time"

// Image is one row of a product's gallery (product_images). URL and
// PublicID stay empty while the async upload is queued. Position is the
// admin-visible gallery order.
type Image struct {
	ID        string
	ProductID string
	URL       string
	PublicID  string
	Position  int
	CreatedAt time.Time
}
