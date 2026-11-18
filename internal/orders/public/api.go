// Package public is the orders module's external contract. Cart hands
// checkout here; payments confirms or cancels through here. Events
// (below) fan side effects out to future subscribers.
package public

import (
	"context"
	"time"

	"github.com/nikhea/malawi-e-commerce-store/pkg/pagination"
)

// Order statuses. pending → paid | cancelled. Terminal states never move.
const (
	StatusPending   = "pending"
	StatusPaid      = "paid"
	StatusCancelled = "cancelled"
)

// CheckoutSessionTTL is how long a Stripe-hosted Checkout session stays
// payable. Stripe requires 30m–24h from session creation; the extra minute
// keeps us clear of clock-skew rejections at the lower bound.
const CheckoutSessionTTL = 31 * time.Minute

// ReserveTTL is the stock-hold lifetime. It outlives CheckoutSessionTTL so
// a payment made in the session's final seconds still has a live hold when
// the webhook arrives; the expiry sweep only sees genuinely abandoned
// checkouts.
const ReserveTTL = CheckoutSessionTTL + 10*time.Minute

// OrderLine is the legal record of one purchased line — snapshotted from
// the cart at checkout, immune to later catalog or cart changes.
type OrderLine struct {
	ProductID      string  `json:"product_id"`
	VariantID      *string `json:"variant_id,omitempty"`
	Name           string  `json:"name"`
	SKU            string  `json:"sku,omitempty"`
	UnitPriceCents int64   `json:"unit_price_cents"`
	Qty            int     `json:"qty"`
	LineTotalCents int64   `json:"line_total_cents"`
}

// Order is the checkout view: header + frozen lines.
type Order struct {
	ID            string      `json:"id"`
	UserID        string      `json:"user_id"`
	Status        string      `json:"status" example:"pending"`
	SubtotalCents int64       `json:"subtotal_cents"`
	Currency      string      `json:"currency" example:"USD"`
	Lines         []OrderLine `json:"lines"`
	CreatedAt     time.Time   `json:"created_at"`
}

// ListFilter scopes the caller's paged order history.
type ListFilter struct {
	// Status narrows to pending, paid or cancelled. Empty = every status.
	Status string
	// Currency narrows to one ISO code (case-insensitive). Empty = all.
	Currency      string
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	MinTotalCents *int64
	MaxTotalCents *int64
	// Sort: newest, oldest, total_asc, total_desc. Empty means newest.
	Sort   string
	Page   int
	Limit  int
	Offset int
}

// Page is one order-history page.
type Page struct {
	Items []Order
	Meta  pagination.Meta
}

// OrderListResponse is the documented wire shape for GET /orders.
type OrderListResponse struct {
	Data []Order         `json:"data"`
	Meta pagination.Meta `json:"meta"`
}

// TopProductsLimit caps the product breakdown returned by the aggregate
// endpoints. Totals always cover every line; only the ranked list is capped.
const TopProductsLimit = 10

// AggregateFilter scopes the order aggregate. Empty status/currency means
// all; date bounds are inclusive. Currency is case-insensitive.
type AggregateFilter struct {
	Status        string
	Currency      string
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
}

// StatusTotal is one status/currency bucket. Amounts stay in the order's
// own currency — MWK and USD are never added together.
type StatusTotal struct {
	Status       string `json:"status" example:"paid"`
	Currency     string `json:"currency" example:"USD"`
	Orders       int    `json:"orders" example:"5"`
	TotalCents   int64  `json:"total_cents" example:"12500"`
	AverageCents int64  `json:"average_cents" example:"2500"`
}

// CurrencyTotal is one currency's order totals.
type CurrencyTotal struct {
	Currency     string `json:"currency" example:"USD"`
	Orders       int    `json:"orders" example:"5"`
	TotalCents   int64  `json:"total_cents" example:"12500"`
	AverageCents int64  `json:"average_cents" example:"2500"`
}

// OrdersAggregate summarizes the matching order headers.
type OrdersAggregate struct {
	Count        int             `json:"count" example:"5"`
	FirstOrderAt *time.Time      `json:"first_order_at,omitempty"`
	LastOrderAt  *time.Time      `json:"last_order_at,omitempty"`
	ByStatus     []StatusTotal   `json:"by_status"`
	ByCurrency   []CurrencyTotal `json:"by_currency"`
}

// ProductTotal is one product's contribution to the matching orders, in
// one currency. Orders counts distinct order headers containing the product.
type ProductTotal struct {
	ProductID    string `json:"product_id"`
	Name         string `json:"name"`
	SKU          string `json:"sku,omitempty"`
	Currency     string `json:"currency" example:"USD"`
	UnitsSold    int    `json:"units_sold" example:"12"`
	Orders       int    `json:"orders" example:"4"`
	RevenueCents int64  `json:"revenue_cents" example:"12000"`
}

// CurrencyRevenue is one currency's line-item totals.
type CurrencyRevenue struct {
	Currency     string `json:"currency" example:"USD"`
	Lines        int    `json:"lines" example:"12"`
	UnitsSold    int    `json:"units_sold" example:"23"`
	RevenueCents int64  `json:"revenue_cents" example:"12500"`
}

// ItemsAggregate summarizes the lines of the matching orders. RevenueByCurrency
// covers every line; TopProducts is ranked by units sold and capped at
// TopProductsLimit.
type ItemsAggregate struct {
	Lines             int               `json:"lines" example:"12"`
	UnitsSold         int               `json:"units_sold" example:"23"`
	RevenueByCurrency []CurrencyRevenue `json:"revenue_by_currency"`
	TopProducts       []ProductTotal    `json:"top_products"`
}

// Aggregates is the combined response for GET /orders/aggregates and
// GET /admin/orders/aggregates.
type Aggregates struct {
	Orders OrdersAggregate `json:"orders"`
	Items  ItemsAggregate  `json:"items"`
}

// AggregatesResponse is the documented wire shape for both aggregate routes.
type AggregatesResponse struct {
	Data Aggregates `json:"data"`
}

// Service is the orders capability surface. Owner-scoped methods take
// userID (from the JWT); MarkPaid/CancelByRef take the order ref for the
// payments module (sync path until payment webhooks land).
type Service interface {
	// Checkout freezes the cart into a pending order, reserves stock,
	// clears the cart, and notifies OrderCreated.
	Checkout(ctx context.Context, userID string) (Order, error)
	// GetByID returns one order, owner-checked.
	GetByID(ctx context.Context, userID, orderID string) (Order, error)
	// GetByRef returns one order WITHOUT owner check. Trusted internal
	// callers only (worker notifications) — never expose via HTTP.
	GetByRef(ctx context.Context, orderID string) (Order, error)
	// ListMine returns one page of the caller's orders, newest first by
	// default.
	ListMine(ctx context.Context, userID string, f ListFilter) (Page, error)
	// AggregateMine summarizes only the caller's orders and their lines.
	AggregateMine(ctx context.Context, userID string, f AggregateFilter) (Aggregates, error)
	// AggregateAll summarizes every order, optionally narrowed to one
	// customer. Admin only: the admin gate lives in the HTTP layer, so
	// non-admin callers must never reach this method.
	AggregateAll(ctx context.Context, userID string, f AggregateFilter) (Aggregates, error)
	// Cancel releases stock and notifies OrderCancelled. Pending only.
	Cancel(ctx context.Context, userID, orderID string) (Order, error)
	// MarkPaid confirms stock and notifies OrderPaid. For payments.
	// Idempotent: paying twice succeeds (webhook retries demand it).
	MarkPaid(ctx context.Context, orderRef string) (Order, error)
	// ExtendCheckoutHold refreshes a pending order's stock hold to the
	// reservation TTL and returns the new deadline. Called when a hosted
	// checkout session is created so the hold outlives the session.
	ExtendCheckoutHold(ctx context.Context, orderID string) (time.Time, error)
	// CancelByRef releases stock for a ref (failed payments). Idempotent.
	CancelByRef(ctx context.Context, orderRef string) error
	// SettleExpired resolves a hold whose TTL passed. It is status-aware:
	// pending orders are cancelled and released, paid orders have the hold
	// confirmed (the payment won the race), cancelled orders just release.
	// The worker calls it per ref; convergence relies on it being safe to
	// repeat.
	SettleExpired(ctx context.Context, orderRef string) error
}
