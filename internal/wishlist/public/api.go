// Package public is the wishlist module's external contract. Nobody
// depends on wishlist (it's a leaf); it reads users + products.
package public

import (
	"context"
	"time"
)

// ProductSummary is the flat catalog snapshot embedded in each item.
// A local copy (not products.Product) so generated API docs resolve —
// same cross-public nesting rule as auth's TokenPair.
type ProductSummary struct {
	ID         string `json:"id"`
	Name       string `json:"name" example:"Sunseed Cooking Oil 2L"`
	Slug       string `json:"slug" example:"sunseed-cooking-oil-2l"`
	PriceCents int64  `json:"price_cents" example:"12500"`
	Currency   string `json:"currency" example:"MWK"`
	ImageURL   string `json:"image_url,omitempty"`
}

// Item is one hearted product with its catalog snapshot attached.
type Item struct {
	ProductID string         `json:"product_id"`
	Product   ProductSummary `json:"product"`
	AddedAt   time.Time      `json:"added_at"`
}

// Service is the wishlist capability surface. All identity from the JWT.
type Service interface {
	// Add hearts a product. Idempotent: re-hearting succeeds silently.
	Add(ctx context.Context, userID, productID string) (Item, error)
	// Remove unhearts. Unknown link = NOT_FOUND.
	Remove(ctx context.Context, userID, productID string) error
	// ListMine returns the caller's wishlist, newest first.
	ListMine(ctx context.Context, userID string) ([]Item, error)
}
