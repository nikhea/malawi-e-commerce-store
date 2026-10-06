// Package public is the categories module's external contract. It is the
// ONLY categories package other modules and cmd/* may import. Products
// reads this to attach catalog entries to the taxonomy.
package public

import (
	"context"
	"time"
)

// Category is the cross-module view of a taxonomy node. ParentID nil =
// root category. ImageURL empty = no image (images are OPTIONAL).
type Category struct {
	ID          string    `json:"id" example:"3fa85f64-5717-4562-b3fc-2c963f66afa6"`
	Name        string    `json:"name" example:"Cooking Oil"`
	Slug        string    `json:"slug" example:"cooking-oil"`
	Description string    `json:"description" example:"Everyday essentials"`
	ParentID    *string   `json:"parent_id,omitempty"`
	ImageURL    string    `json:"image_url,omitempty" example:"https://res.cloudinary.com/…/oil.jpg"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreateCategoryInput carries a new node. Slug empty = generated from
// Name. ParentID nil = root. ImageURL empty = no image yet (upload later
// via the async image endpoint).
type CreateCategoryInput struct {
	Name        string
	Slug        string
	Description string
	ParentID    *string
	ImageURL    string
}

// UpdateCategoryInput carries editable fields. Nil = leave unchanged.
type UpdateCategoryInput struct {
	Name        *string
	Description *string
	ParentID    *string
	// ClearParent detaches the node back to root (ParentID nil alone
	// can't express "no change" vs "detach").
	ClearParent bool
}

// Service is the categories capability surface. Reads are public
// (storefront browsing); writes are admin-gated at the routes layer.
type Service interface {
	Create(ctx context.Context, in CreateCategoryInput) (Category, error)
	GetByID(ctx context.Context, id string) (Category, error)
	GetBySlug(ctx context.Context, slug string) (Category, error)
	List(ctx context.Context) ([]Category, error)
	Update(ctx context.Context, id string, in UpdateCategoryInput) (Category, error)
	Delete(ctx context.Context, id string) error
	// SetImage stores the finished upload outcome. Called by admins
	// (direct link) and by the media pipeline callback (async upload) —
	// both funnel here so the write path stays single.
	SetImage(ctx context.Context, id, url, publicID string) (Category, error)
}
