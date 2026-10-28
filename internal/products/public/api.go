// Package public is the products module's external contract. It is the
// ONLY products package other modules and cmd/* may import. Cart, orders,
// wishlist and reviews read this; variants rows are composed into Detail
// (as flat VariantView scalars — swagger can't resolve types nested
// across two public packages, same rule as auth's TokenPair).
package public

import (
	"context"
	"time"

	"github.com/nikhea/malawi-e-commerce-store/pkg/pagination"
)

// Product is the catalog view of an SPU. Prices are USD cents:
// 2500 = $25.00. Never floats for money.
type Product struct {
	ID           string    `json:"id" example:"3fa85f64-5717-4562-b3fc-2c963f66afa6"`
	CategoryID   *string   `json:"category_id,omitempty"`
	CategoryName string    `json:"category_name,omitempty" example:"Cooking Oil"`
	Name         string    `json:"name" example:"Sunseed Cooking Oil"`
	Slug         string    `json:"slug" example:"sunseed-cooking-oil"`
	Description  string    `json:"description" example:"Pure sunflower oil"`
	PriceCents   int64     `json:"price_cents" example:"12500"`
	Currency     string    `json:"currency" example:"USD"`
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
	Available  int    `json:"available" example:"16"`
	InStock    bool   `json:"in_stock" example:"true"`
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
// first page, name order (relevance when Query is set).
type ListFilter struct {
	// Query runs full-text search over name, description and slug. Word
	// prefixes match ("cook" finds "cooking"); when nothing matches, the
	// repository falls back to case-insensitive substring search.
	Query string
	// CategoryID and CategorySlug are alternative taxonomy filters.
	CategoryID    *string
	CategorySlug  *string
	MinPriceCents *int64
	MaxPriceCents *int64
	// InStock keeps only products with at least one sellable unit.
	InStock *bool
	// Size matches an active variant name (XS, M, XL, ...).
	Size string
	// MinRating keeps products whose average review rating is at least
	// this (0-5). Products without reviews are excluded.
	MinRating     *float64
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	ActiveOnly    bool
	// Sort is a whitelisted order key; empty means "relevance" when Query
	// is set, otherwise "name_asc".
	Sort   string
	Page   int
	Limit  int
	Offset int
}

// Page is one catalog page: rows plus the counters clients need to page
// without a second count request.
type Page struct {
	Items []Product
	Meta  pagination.Meta
}

// ProductListResponse is the documented wire shape for GET /products:
// the array stays under "data", paging counters under "meta".
type ProductListResponse struct {
	Data []Product       `json:"data"`
	Meta pagination.Meta `json:"meta"`
}

// Service is the products capability surface. Reads are public;
// writes are admin-gated at the routes layer.
type Service interface {
	Create(ctx context.Context, in CreateProductInput) (Product, error)
	GetByID(ctx context.Context, id string) (Detail, error)
	GetBySlug(ctx context.Context, slug string) (Detail, error)
	List(ctx context.Context, f ListFilter) (Page, error)
	Update(ctx context.Context, id string, in UpdateProductInput) (Product, error)
	Delete(ctx context.Context, id string) error
	SetImage(ctx context.Context, id, url, publicID string) (Product, error)
}
