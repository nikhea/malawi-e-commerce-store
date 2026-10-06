// Package public is the products module's external contract. It is the
// ONLY products package other modules and cmd/* may import. Cart, orders,
// wishlist and reviews read this; variants rows are composed into Detail
// (as flat VariantView scalars — swagger can't resolve types nested
// across two public packages, same rule as auth's TokenPair).
package public

import (
	"context"
	"time"
)

// Product is the catalog view of an SPU. Prices are minor units
// (tambala): 2500 = K25.00. Never floats for money.
type Product struct {
	ID           string    `json:"id" example:"3fa85f64-5717-4562-b3fc-2c963f66afa6"`
	CategoryID   *string   `json:"category_id,omitempty"`
	CategoryName string    `json:"category_name,omitempty" example:"Cooking Oil"`
	Name         string    `json:"name" example:"Sunseed Cooking Oil"`
	Slug         string    `json:"slug" example:"sunseed-cooking-oil"`
	Description  string    `json:"description" example:"Pure sunflower oil"`
	PriceCents   int64     `json:"price_cents" example:"12500"`
	Currency     string    `json:"currency" example:"MWK"`
	ImageURL     string    `json:"image_url,omitempty"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
}

// VariantView is the flat SKU view embedded in Detail. Mirrors
// variants/public.Variant field-for-field as scalars.
type VariantView struct {
	ID         string `json:"id"`
	Name       string `json:"name" example:"500ml"`
	SKU        string `json:"sku" example:"OIL-500-001"`
	PriceCents int64  `json:"price_cents" example:"2500"`
	IsActive   bool   `json:"is_active"`
}

// Detail is the storefront product page: product + its SKUs.
type Detail struct {
	Product  Product       `json:"product"`
	Variants []VariantView `json:"variants"`
}

// CreateProductInput carries a new SPU. CategoryID nil = unfiled.
type CreateProductInput struct {
	CategoryID  *string
	Name        string
	Slug        string
	Description string
	PriceCents  int64
	Currency    string
	ImageURL    string
}

// UpdateProductInput carries editable fields. Nil = leave unchanged.
type UpdateProductInput struct {
	CategoryID  *string
	Name        *string
	Description *string
	PriceCents  *int64
	Currency    *string
	IsActive    *bool
	// ClearCategory unfiles the product (CategoryID nil alone can't
	// express "no change" vs "detach").
	ClearCategory bool
}

// ListFilter scopes the catalog listing. Zero value = active only,
// first page.
type ListFilter struct {
	CategoryID *string
	ActiveOnly bool
	Limit      int
	Offset     int
}

// Service is the products capability surface. Reads are public;
// writes are admin-gated at the routes layer.
type Service interface {
	Create(ctx context.Context, in CreateProductInput) (Product, error)
	GetByID(ctx context.Context, id string) (Detail, error)
	GetBySlug(ctx context.Context, slug string) (Detail, error)
	List(ctx context.Context, f ListFilter) ([]Product, error)
	Update(ctx context.Context, id string, in UpdateProductInput) (Product, error)
	Delete(ctx context.Context, id string) error
	SetImage(ctx context.Context, id, url, publicID string) (Product, error)
}
