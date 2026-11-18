package model

import "time"

// Order is the orders table row. Module-private.
type Order struct {
	ID            string
	UserID        string
	Status        string
	SubtotalCents int64
	Currency      string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Item is an order_items row. Module-private.
type Item struct {
	ID             string
	OrderID        string
	ProductID      string
	VariantID      *string
	Name           string
	SKU            string
	UnitPriceCents int64
	Qty            int
}

// ListFilter scopes the caller's paged order history.
type ListFilter struct {
	Status        string
	Currency      string
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	MinTotalCents *int64
	MaxTotalCents *int64
	Sort          string
	Limit         int
	Offset        int
}

// AggregateFilter scopes an aggregate over orders. An empty userID means
// every customer (admin only); Status/Currency/date bounds narrow both the
// order summary and the line-item breakdown.
type AggregateFilter struct {
	Status        string
	Currency      string
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	TopProducts   int
}

// StatusTotal is one status/currency bucket of order headers. Money is kept
// per currency: legacy MWK orders and current USD orders must never be
// summed into one meaningless number.
type StatusTotal struct {
	Status       string
	Currency     string
	Orders       int
	TotalCents   int64
	AverageCents int64
}

// CurrencyTotal is one currency's order-header totals.
type CurrencyTotal struct {
	Currency     string
	Orders       int
	TotalCents   int64
	AverageCents int64
}

// OrdersAggregate summarizes the matching order headers.
type OrdersAggregate struct {
	Count        int
	FirstOrderAt *time.Time
	LastOrderAt  *time.Time
	ByStatus     []StatusTotal
	ByCurrency   []CurrencyTotal
}

// ProductTotal is one product's line-item totals within a currency.
type ProductTotal struct {
	ProductID    string
	Name         string
	SKU          string
	Currency     string
	UnitsSold    int
	Orders       int
	RevenueCents int64
}

// CurrencyRevenue is one currency's line-item totals.
type CurrencyRevenue struct {
	Currency     string
	Lines        int
	UnitsSold    int
	RevenueCents int64
}

// ItemsAggregate summarizes the lines of the matching orders.
type ItemsAggregate struct {
	Lines             int
	UnitsSold         int
	RevenueByCurrency []CurrencyRevenue
	TopProducts       []ProductTotal
}

// Aggregates is the combined order + line-item summary.
type Aggregates struct {
	Orders OrdersAggregate
	Items  ItemsAggregate
}
