// Package public is the dashboard module's external contract. The dashboard
// is a read-only reporting projection: it never mutates state and exposes a
// single admin-only overview snapshot.
package public

import (
	"context"
	"time"
)

// TopItemsLimit caps each demand ranking. Totals elsewhere are unaffected.
const TopItemsLimit = 5

// Catalog counts describe the live product catalog.
type Catalog struct {
	TotalProducts    int64 `json:"total_products"`
	ActiveProducts   int64 `json:"active_products"`
	InactiveProducts int64 `json:"inactive_products"`
	TotalVariants    int64 `json:"total_variants"`
	ActiveVariants   int64 `json:"active_variants"`
	TotalCategories  int64 `json:"total_categories"`
	ReadyImages      int64 `json:"ready_images"`
}

// Customers counts accounts and buying behaviour. PayingCustomers counts
// distinct users with a paid order; SucceededPaymentCustomers counts those
// with a succeeded payment ledger row. They can differ when a payment is
// recorded without the order transition (webhook retry window).
type Customers struct {
	TotalUsers                int64 `json:"total_users"`
	TotalCustomers            int64 `json:"total_customers"`
	VerifiedCustomers         int64 `json:"verified_customers"`
	TotalAdmins               int64 `json:"total_admins"`
	NewCustomers30D           int64 `json:"new_customers_30d"`
	PayingCustomers           int64 `json:"paying_customers"`
	SucceededPaymentCustomers int64 `json:"succeeded_payment_customers"`
}

// CurrencyOrderTotal keeps money in its own currency. Historical MWK orders
// must never be summed with current USD orders.
type CurrencyOrderTotal struct {
	Currency   string `json:"currency" example:"USD"`
	Orders     int64  `json:"orders"`
	TotalCents int64  `json:"total_cents"`
}

// Orders summarizes the order headers. There is no shipped/delivered status
// in this schema: pending → paid | cancelled is the whole order state machine.
type Orders struct {
	Total                 int64                `json:"total"`
	Pending               int64                `json:"pending"`
	Paid                  int64                `json:"paid"`
	Cancelled             int64                `json:"cancelled"`
	Last30D               int64                `json:"last_30d"`
	ByCurrency            []CurrencyOrderTotal `json:"by_currency"`
	PaidRevenueByCurrency []CurrencyOrderTotal `json:"paid_revenue_by_currency"`
}

// Fulfillment reports inventory reservations, the project's closest proxy
// for a fulfillment pipeline (orders themselves have no shipping states).
// ExpiredActiveHolds are active holds past their TTL awaiting the worker's
// expiry sweep.
type Fulfillment struct {
	ActiveReservations    int64 `json:"active_reservations"`
	ConfirmedReservations int64 `json:"confirmed_reservations"`
	ReleasedReservations  int64 `json:"released_reservations"`
	ActiveReservedQty     int64 `json:"active_reserved_qty"`
	ConfirmedQty          int64 `json:"confirmed_qty"`
	ReleasedQty           int64 `json:"released_qty"`
	ExpiredActiveHolds    int64 `json:"expired_active_holds"`
}

// StockHealth is the on-hand ledger health. OutOfStockRows have reserved_qty
// >= on_hand_qty (the CHECK constraint allows equality).
type StockHealth struct {
	Rows           int64 `json:"rows"`
	OnHandQty      int64 `json:"on_hand_qty"`
	ReservedQty    int64 `json:"reserved_qty"`
	AvailableQty   int64 `json:"available_qty"`
	OutOfStockRows int64 `json:"out_of_stock_rows"`
}

// Reviews aggregates every review, not just one product's.
type Reviews struct {
	Total         int64   `json:"total"`
	AverageRating float64 `json:"average_rating"`
	Last30D       int64   `json:"last_30d"`
}

// PaymentStatusTotal is one status/currency bucket of the Stripe ledger.
type PaymentStatusTotal struct {
	Status             string `json:"status" example:"succeeded"`
	Currency           string `json:"currency" example:"USD"`
	Payments           int64  `json:"payments"`
	ChargedCents       int64  `json:"charged_cents"`
	OriginalOrderCents int64  `json:"original_order_cents"`
}

// Payments summarizes the ledger by status and currency.
type Payments struct {
	ByStatus []PaymentStatusTotal `json:"by_status"`
}

// OrderedProduct is a paid-order line ranking (realized demand).
type OrderedProduct struct {
	ProductID    string `json:"product_id"`
	Name         string `json:"name"`
	SKU          string `json:"sku,omitempty"`
	Currency     string `json:"currency" example:"USD"`
	UnitsSold    int64  `json:"units_sold"`
	Orders       int64  `json:"orders"`
	RevenueCents int64  `json:"revenue_cents"`
}

// WishlistedProduct is a wishlist-heart ranking (stated demand).
type WishlistedProduct struct {
	ProductID string `json:"product_id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Wishlists int64  `json:"wishlists"`
}

// CartedProduct is a live-cart ranking (current demand).
type CartedProduct struct {
	ProductID string `json:"product_id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	CartQty   int64  `json:"cart_qty"`
	Carts     int64  `json:"carts"`
}

// Demand holds the three "most required items" rankings.
type Demand struct {
	MostOrdered    []OrderedProduct    `json:"most_ordered"`
	MostWishlisted []WishlistedProduct `json:"most_wishlisted"`
	MostCarted     []CartedProduct     `json:"most_carted"`
}

// Overview is the single admin dashboard snapshot.
type Overview struct {
	GeneratedAt time.Time   `json:"generated_at"`
	Catalog     Catalog     `json:"catalog"`
	Customers   Customers   `json:"customers"`
	Orders      Orders      `json:"orders"`
	Fulfillment Fulfillment `json:"fulfillment"`
	Stock       StockHealth `json:"stock"`
	Reviews     Reviews     `json:"reviews"`
	Payments    Payments    `json:"payments"`
	Demand      Demand      `json:"demand"`
}

// OverviewResponse is the documented wire shape for the overview route.
type OverviewResponse struct {
	Data Overview `json:"data"`
}

// Service is the dashboard capability surface. Overview takes no filters:
// it is a whole-store snapshot for admins only.
type Service interface {
	Overview(ctx context.Context) (Overview, error)
}
