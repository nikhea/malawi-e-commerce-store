// Package model holds the dashboard's module-private persistence shapes.
package model

type Catalog struct {
	TotalProducts    int64
	ActiveProducts   int64
	InactiveProducts int64
	TotalVariants    int64
	ActiveVariants   int64
	TotalCategories  int64
	ReadyImages      int64
}

type Customers struct {
	TotalUsers                int64
	TotalCustomers            int64
	VerifiedCustomers         int64
	TotalAdmins               int64
	NewCustomers30D           int64
	PayingCustomers           int64
	SucceededPaymentCustomers int64
}

type CurrencyOrderTotal struct {
	Currency   string
	Orders     int64
	TotalCents int64
}

type Orders struct {
	Total                 int64
	Pending               int64
	Paid                  int64
	Cancelled             int64
	Last30D               int64
	ByCurrency            []CurrencyOrderTotal
	PaidRevenueByCurrency []CurrencyOrderTotal
}

type Fulfillment struct {
	ActiveReservations    int64
	ConfirmedReservations int64
	ReleasedReservations  int64
	ActiveReservedQty     int64
	ConfirmedQty          int64
	ReleasedQty           int64
	ExpiredActiveHolds    int64
}

type StockHealth struct {
	Rows           int64
	OnHandQty      int64
	ReservedQty    int64
	AvailableQty   int64
	OutOfStockRows int64
}

type Reviews struct {
	Total         int64
	AverageRating float64
	Last30D       int64
}

type PaymentStatusTotal struct {
	Status             string
	Currency           string
	Payments           int64
	ChargedCents       int64
	OriginalOrderCents int64
}

type Payments struct {
	ByStatus []PaymentStatusTotal
}

type OrderedProduct struct {
	ProductID    string
	Name         string
	SKU          string
	Currency     string
	UnitsSold    int64
	Orders       int64
	RevenueCents int64
}

type WishlistedProduct struct {
	ProductID string
	Name      string
	Slug      string
	Wishlists int64
}

type CartedProduct struct {
	ProductID string
	Name      string
	Slug      string
	CartQty   int64
	Carts     int64
}

type Demand struct {
	MostOrdered    []OrderedProduct
	MostWishlisted []WishlistedProduct
	MostCarted     []CartedProduct
}

type Overview struct {
	Catalog     Catalog
	Customers   Customers
	Orders      Orders
	Fulfillment Fulfillment
	Stock       StockHealth
	Reviews     Reviews
	Payments    Payments
	Demand      Demand
}

// OverviewOptions carries the read policy. topItems caps each demand
// ranking; it never affects the summary counters.
type OverviewOptions struct {
	TopItems int
}
