// Package public is the reviews module's external contract. Writes never
// block the catalog read path: reviews attach to products, products never
// reads reviews back.
package public

import (
	"context"
	"time"
)

// Review is one customer's verdict. Author snapshots users.name at write
// time so later renames don't rewrite history.
type Review struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Author    string    `json:"author" example:"Aisha B."`
	ProductID string    `json:"product_id"`
	Rating    int       `json:"rating" example:"5"`
	Title     string    `json:"title" example:"Great oil"`
	Body      string    `json:"body" example:"Lasted the whole month"`
	CreatedAt time.Time `json:"created_at"`
}

// Summary is the storefront aggregate + page.
type Summary struct {
	Average float64  `json:"average" example:"4.5"`
	Count   int      `json:"count" example:"12"`
	Reviews []Review `json:"reviews"`
}

// CreateReviewInput carries a new verdict. One per user per product.
type CreateReviewInput struct {
	UserID    string
	ProductID string
	Rating    int
	Title     string
	Body      string
}

// UpdateReviewInput carries editable fields. Nil = leave unchanged.
type UpdateReviewInput struct {
	Rating *int
	Title  *string
	Body   *string
}

// Service is the reviews capability surface.
type Service interface {
	// Create writes one review per user per product (second write is a
	// CONFLICT — edit instead).
	Create(ctx context.Context, in CreateReviewInput) (Review, error)
	// UpdateMine edits the caller's own review.
	UpdateMine(ctx context.Context, userID, reviewID string, in UpdateReviewInput) (Review, error)
	// DeleteMine removes the caller's own review.
	DeleteMine(ctx context.Context, userID, reviewID string) error
	// DeleteAny removes any review (admin moderation).
	DeleteAny(ctx context.Context, reviewID string) error
	// Summary returns the public aggregate + review page for a product.
	Summary(ctx context.Context, productID string) (Summary, error)
}
