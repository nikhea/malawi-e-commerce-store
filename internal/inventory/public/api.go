// Package public is the inventory module's external contract. Orders
// reserves stock through here at checkout; payments confirms or releases
// via events. Absolute quantities only — never deltas without a reason.
package public

import (
	"context"
	"time"

	bulkdto "github.com/nikhea/malawi-e-commerce-store/pkg/bulk/dto"
)

// Stock is the availability view. Available = OnHand - Reserved.
type Stock struct {
	ProductID string  `json:"product_id"`
	VariantID *string `json:"variant_id,omitempty"`
	OnHand    int     `json:"on_hand" example:"50"`
	Reserved  int     `json:"reserved" example:"3"`
	Available int     `json:"available" example:"47"`
}

// BulkStockItem is one set-stock line. Stock is keyed by product plus
// variant, so both ids travel together (the spec's "variantId" shorthand).
type BulkStockItem struct {
	ProductID string
	VariantID *string
	Qty       int
}

// ReserveLine is one SKU slice of a reservation.
type ReserveLine struct {
	ProductID string
	VariantID *string
	Qty       int
}

// Service is the inventory capability surface.
type Service interface {
	// SetStock upserts the on-hand quantity (admin restocking).
	SetStock(ctx context.Context, productID string, variantID *string, qty int) (Stock, error)
	// BulkSetStock sets on-hand stock for many SKUs. With atomic=true every
	// line commits or the batch rolls back; otherwise failures are per item.
	BulkSetStock(ctx context.Context, items []BulkStockItem, atomic bool) ([]bulkdto.BulkResult[Stock], error)
	// GetStockByVariants reads availability for up to MAX_BULK_ITEMS
	// variants. Unknown ids are returned in notFound; known variants with no
	// inventory row read as zero stock.
	GetStockByVariants(ctx context.Context, variantIDs []string) ([]Stock, []string, error)
	// GetStock reads availability. Unknown SKU = zero stock, NOT an
	// error (untracked items simply can't be reserved).
	GetStock(ctx context.Context, productID string, variantID *string) (Stock, error)
	// ListStockByProduct returns every stock row for a product, in one
	// query. Callers compose it into catalog reads without N+1 lookups.
	ListStockByProduct(ctx context.Context, productID string) ([]Stock, error)
	// Reserve holds qty for orderRef for ttl. Fails with CONFLICT when
	// any line lacks availability. Atomic: all lines or none.
	Reserve(ctx context.Context, orderRef string, lines []ReserveLine, ttl time.Duration) error
	// Release frees an order's active holds (cancel/expire path).
	ReleaseByOrder(ctx context.Context, orderRef string) error
	// ConfirmByOrder converts holds into sales (payment path).
	ConfirmByOrder(ctx context.Context, orderRef string) error
	// ReleaseExpired frees all holds past their TTL and returns the
	// affected order refs (worker expiry sweep; orders cancels them).
	ReleaseExpired(ctx context.Context) ([]string, error)
}
