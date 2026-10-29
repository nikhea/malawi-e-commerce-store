// Package public is the wishlist module's external contract. Nobody
// depends on wishlist (it's a leaf); it reads users + products.
package public

import (
	"context"
	"time"

	"github.com/nikhea/malawi-e-commerce-store/pkg/pagination"
)

// ProductSummary is the flat catalog snapshot embedded in each item.
// A local copy (not products.Product) so generated API docs resolve —
// same cross-public nesting rule as auth's TokenPair.
type ProductSummary struct {
	ID         string `json:"id"`
	Name       string `json:"name" example:"Sunseed Cooking Oil 2L"`
	Slug       string `json:"slug" example:"sunseed-cooking-oil-2l"`
	PriceCents int64  `json:"price_cents" example:"12500"`
	Currency   string `json:"currency" example:"USD"`
	ImageURL   string `json:"image_url,omitempty"`
}

// Item is one hearted product with its catalog snapshot attached.
type Item struct {
	ProductID string         `json:"product_id"`
	Product   ProductSummary `json:"product"`
	AddedAt   time.Time      `json:"added_at"`
}

// ListFilter scopes the caller's wishlist page.
type ListFilter struct {
	// Query matches product name or slug (case-insensitive substring).
	Query string
	// CategoryID narrows to products in one category.
	CategoryID *string
	// Sort: newest, oldest, name_asc, name_desc, price_asc, price_desc.
	// Empty means newest.
	Sort   string
	Page   int
	Limit  int
	Offset int
}

// Page is one wishlist page.
type Page struct {
	Items []Item
	Meta  pagination.Meta
}

// WishlistListResponse is the documented wire shape for GET /wishlist.
type WishlistListResponse struct {
	Data []Item          `json:"data"`
	Meta pagination.Meta `json:"meta"`
}

// Service is the wishlist capability surface. All identity from the JWT.
type Service interface {
	// Add hearts a product. Idempotent: re-hearting succeeds silently.
	Add(ctx context.Context, userID, productID string) (Item, error)
	// Remove unhearts. Unknown link = NOT_FOUND.
	Remove(ctx context.Context, userID, productID string) error
	// ListMine returns one page of the caller's wishlist, newest first
	// by default.
	ListMine(ctx context.Context, userID string, f ListFilter) (Page, error)
}
