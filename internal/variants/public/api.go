// Package public is the variants module's external contract. Variants are
// SKU-level rows (size/flavor/pack) owned conceptually by products; the
// products module composes these into its detail view.
package public

import (
	"context"
	"time"
)

// Variant is one sellable SKU. Price is absolute minor units (tambala),
// NOT a delta — no base+delta math bugs at checkout.
type Variant struct {
	ID         string    `json:"id" example:"3fa85f64-5717-4562-b3fc-2c963f66afa6"`
	ProductID  string    `json:"product_id"`
	Name       string    `json:"name" example:"500ml"`
	SKU        string    `json:"sku" example:"OIL-500-001"`
	PriceCents int64     `json:"price_cents" example:"2500"`
	IsActive   bool      `json:"is_active"`
	CreatedAt  time.Time `json:"created_at"`
}

// CreateVariantInput carries a new SKU. Product existence is enforced by
// the DB foreign key (mapped to NOT_FOUND), so this module imports no
// sibling — the leaf stays a leaf.
type CreateVariantInput struct {
	ProductID  string
	Name       string
	SKU        string
	PriceCents int64
}

// UpdateVariantInput carries editable fields. Nil = leave unchanged.
type UpdateVariantInput struct {
	Name       *string
	PriceCents *int64
	IsActive   *bool
}

// Service is the variants capability surface.
type Service interface {
	Create(ctx context.Context, in CreateVariantInput) (Variant, error)
	ListByProduct(ctx context.Context, productID string) ([]Variant, error)
	Update(ctx context.Context, id string, in UpdateVariantInput) (Variant, error)
	Delete(ctx context.Context, id string) error
}
