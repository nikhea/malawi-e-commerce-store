// Package public is the reviews module's external contract. Writes never
// block the catalog read path: reviews attach to products, products never
// reads reviews back.
package public

import (
	"context"
	"time"

	"github.com/nikhea/malawi-e-commerce-store/pkg/pagination"
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

// Summary is the storefront aggregate + review page. Count and Average
// describe every review for the product; Reviews and Meta describe the
// (optionally filtered) page.
type Summary struct {
	Average float64         `json:"average" example:"4.5"`
	Count   int             `json:"count" example:"12"`
	Reviews []Review        `json:"reviews"`
	Meta    pagination.Meta `json:"meta"`
}

// ListFilter scopes the review page. The aggregate ignores these filters.
type ListFilter struct {
	// Query matches title, body or author (case-insensitive substring).
	Query string
	// Rating narrows to one 1–5 rating.
	Rating *int
	// Sort: newest, oldest, highest, lowest. Empty means newest.
	Sort   string
	Page   int
	Limit  int
	Offset int
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

// MyReviewProduct is the catalog context attached to a review in the
// caller's own history.
type MyReviewProduct struct {
	ID       string `json:"id"`
	Name     string `json:"name" example:"Sunseed Cooking Oil 2L"`
	Slug     string `json:"slug" example:"sunseed-cooking-oil-2l"`
	ImageURL string `json:"image_url,omitempty"`
}

// MyReview is one of the caller's reviews with product context.
type MyReview struct {
	ID        string          `json:"id"`
	UserID    string          `json:"user_id"`
	ProductID string          `json:"product_id"`
	Rating    int             `json:"rating" example:"5"`
	Title     string          `json:"title" example:"Great oil"`
	Body      string          `json:"body" example:"Lasted the whole month"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	Product   MyReviewProduct `json:"product"`
}

// MyReviewsPage is one page of the caller's reviews.
type MyReviewsPage struct {
	Items []MyReview
	Meta  pagination.Meta
}

// MyReviewsResponse is the documented wire shape for GET /reviews/me.
type MyReviewsResponse struct {
	Data []MyReview      `json:"data"`
	Meta pagination.Meta `json:"meta"`
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
	// Summary returns the public aggregate + one filtered review page.
	Summary(ctx context.Context, productID string, f ListFilter) (Summary, error)
	// ListMine returns the caller's reviews, newest first by default,
	// with product context joined in.
	ListMine(ctx context.Context, userID string, f ListFilter) (MyReviewsPage, error)
}
