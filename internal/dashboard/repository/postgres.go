// Package repository owns the dashboard's read-only SQL. The dashboard is a
// reporting projection over existing tables: it imports no other module's
// internals, so it cannot create a dependency cycle or bypass their public
// contracts. It never writes.
package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nikhea/malawi-e-commerce-store/internal/dashboard/model"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
)

// Postgres is the read-only dashboard store.
type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

// counts is the single-row scalar subquery block. int64 matches Postgres'
// COUNT/SUM return type so no narrowing conversions are needed.
type counts struct {
	totalProducts, activeProducts, inactiveProducts             int64
	totalVariants, activeVariants, totalCategories, readyImages int64
	totalUsers, totalCustomers, verifiedCustomers, totalAdmins  int64
	newCustomers30D, payingCustomers, succeededPaymentCustomers int64
	totalOrders, pendingOrders, paidOrders, cancelledOrders     int64
	ordersLast30D, totalReviews, reviewsLast30D                 int64
	averageRating                                               float64
	stockRows, onHandQty, reservedQty, availableQty             int64
	outOfStockRows                                              int64
	activeRes, confirmedRes, releasedRes                        int64
	activeQty, confirmedQty, releasedQty                        int64
	expiredActiveHolds                                          int64
}

// Overview collects the snapshot in five grouped queries. One wide query
// handles every scalar counter; the others produce the money and demand
// breakdowns that cannot be expressed as a single scalar.
func (r *Postgres) Overview(ctx context.Context, opts model.OverviewOptions) (model.Overview, error) {
	var out model.Overview
	out.Orders.ByCurrency = []model.CurrencyOrderTotal{}
	out.Orders.PaidRevenueByCurrency = []model.CurrencyOrderTotal{}
	out.Payments.ByStatus = []model.PaymentStatusTotal{}
	out.Demand.MostOrdered = []model.OrderedProduct{}
	out.Demand.MostWishlisted = []model.WishlistedProduct{}
	out.Demand.MostCarted = []model.CartedProduct{}

	const scalars = `
SELECT
  (SELECT COUNT(*) FROM products),
  (SELECT COUNT(*) FROM products WHERE is_active),
  (SELECT COUNT(*) FROM products WHERE NOT is_active),
  (SELECT COUNT(*) FROM variants),
  (SELECT COUNT(*) FROM variants WHERE is_active),
  (SELECT COUNT(*) FROM categories),
  (SELECT COUNT(*) FROM product_images WHERE url <> ''),
  (SELECT COUNT(*) FROM users),
  (SELECT COUNT(*) FROM users WHERE role = 'customer'),
  (SELECT COUNT(*) FROM users WHERE role = 'customer' AND email_verified),
  (SELECT COUNT(*) FROM users WHERE role = 'admin'),
  (SELECT COUNT(*) FROM users WHERE role = 'customer' AND created_at >= now() - interval '30 days'),
  (SELECT COUNT(DISTINCT user_id) FROM orders WHERE status = 'paid'),
  (SELECT COUNT(DISTINCT o.user_id) FROM orders o JOIN payments p ON p.order_id = o.id WHERE p.status = 'succeeded'),
  (SELECT COUNT(*) FROM orders),
  (SELECT COUNT(*) FROM orders WHERE status = 'pending'),
  (SELECT COUNT(*) FROM orders WHERE status = 'paid'),
  (SELECT COUNT(*) FROM orders WHERE status = 'cancelled'),
  (SELECT COUNT(*) FROM orders WHERE created_at >= now() - interval '30 days'),
  (SELECT COUNT(*) FROM reviews),
  (SELECT COALESCE(AVG(rating), 0)::double precision FROM reviews),
  (SELECT COUNT(*) FROM reviews WHERE created_at >= now() - interval '30 days'),
  (SELECT COUNT(*) FROM inventory),
  (SELECT COALESCE(SUM(on_hand_qty), 0) FROM inventory),
  (SELECT COALESCE(SUM(reserved_qty), 0) FROM inventory),
  (SELECT COALESCE(SUM(on_hand_qty - reserved_qty), 0) FROM inventory),
  (SELECT COUNT(*) FROM inventory WHERE on_hand_qty <= reserved_qty),
  (SELECT COUNT(*) FROM reservations WHERE status = 'active'),
  (SELECT COUNT(*) FROM reservations WHERE status = 'confirmed'),
  (SELECT COUNT(*) FROM reservations WHERE status = 'released'),
  (SELECT COALESCE(SUM(qty), 0) FROM reservations WHERE status = 'active'),
  (SELECT COALESCE(SUM(qty), 0) FROM reservations WHERE status = 'confirmed'),
  (SELECT COALESCE(SUM(qty), 0) FROM reservations WHERE status = 'released'),
  (SELECT COUNT(DISTINCT order_ref) FROM reservations WHERE status = 'active' AND expires_at < now())`

	var c counts
	err := r.pool.QueryRow(ctx, scalars).Scan(
		&c.totalProducts, &c.activeProducts, &c.inactiveProducts,
		&c.totalVariants, &c.activeVariants, &c.totalCategories, &c.readyImages,
		&c.totalUsers, &c.totalCustomers, &c.verifiedCustomers, &c.totalAdmins,
		&c.newCustomers30D, &c.payingCustomers, &c.succeededPaymentCustomers,
		&c.totalOrders, &c.pendingOrders, &c.paidOrders, &c.cancelledOrders,
		&c.ordersLast30D, &c.totalReviews, &c.averageRating, &c.reviewsLast30D,
		&c.stockRows, &c.onHandQty, &c.reservedQty, &c.availableQty,
		&c.outOfStockRows,
		&c.activeRes, &c.confirmedRes, &c.releasedRes,
		&c.activeQty, &c.confirmedQty, &c.releasedQty,
		&c.expiredActiveHolds,
	)
	if err != nil {
		return model.Overview{}, apperr.Internal(err)
	}
	out.Catalog = model.Catalog{
		TotalProducts: c.totalProducts, ActiveProducts: c.activeProducts,
		InactiveProducts: c.inactiveProducts, TotalVariants: c.totalVariants,
		ActiveVariants: c.activeVariants, TotalCategories: c.totalCategories,
		ReadyImages: c.readyImages,
	}
	out.Customers = model.Customers{
		TotalUsers: c.totalUsers, TotalCustomers: c.totalCustomers,
		VerifiedCustomers: c.verifiedCustomers, TotalAdmins: c.totalAdmins,
		NewCustomers30D: c.newCustomers30D, PayingCustomers: c.payingCustomers,
		SucceededPaymentCustomers: c.succeededPaymentCustomers,
	}
	out.Orders.Total = c.totalOrders
	out.Orders.Pending = c.pendingOrders
	out.Orders.Paid = c.paidOrders
	out.Orders.Cancelled = c.cancelledOrders
	out.Orders.Last30D = c.ordersLast30D
	out.Reviews = model.Reviews{
		Total: c.totalReviews, AverageRating: c.averageRating, Last30D: c.reviewsLast30D,
	}
	out.Stock = model.StockHealth{
		Rows: c.stockRows, OnHandQty: c.onHandQty, ReservedQty: c.reservedQty,
		AvailableQty: c.availableQty, OutOfStockRows: c.outOfStockRows,
	}
	out.Fulfillment = model.Fulfillment{
		ActiveReservations: c.activeRes, ConfirmedReservations: c.confirmedRes,
		ReleasedReservations: c.releasedRes, ActiveReservedQty: c.activeQty,
		ConfirmedQty: c.confirmedQty, ReleasedQty: c.releasedQty,
		ExpiredActiveHolds: c.expiredActiveHolds,
	}

	const currencyTotals = `SELECT upper(currency), COUNT(*), COALESCE(SUM(subtotal_cents), 0)
	                        FROM orders %s
	                        GROUP BY upper(currency) ORDER BY upper(currency)`
	for _, target := range []struct {
		where string
		dest  *[]model.CurrencyOrderTotal
	}{
		{"", &out.Orders.ByCurrency},
		{"WHERE status = 'paid'", &out.Orders.PaidRevenueByCurrency},
	} {
		rows, err := r.pool.Query(ctx, fmt.Sprintf(currencyTotals, target.where))
		if err != nil {
			return model.Overview{}, apperr.Internal(err)
		}
		for rows.Next() {
			var t model.CurrencyOrderTotal
			if err := rows.Scan(&t.Currency, &t.Orders, &t.TotalCents); err != nil {
				rows.Close()
				return model.Overview{}, apperr.Internal(err)
			}
			*target.dest = append(*target.dest, t)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return model.Overview{}, apperr.Internal(err)
		}
		rows.Close()
	}

	paymentRows, err := r.pool.Query(ctx, `
SELECT status, upper(currency), COUNT(*), COALESCE(SUM(amount_cents), 0),
       COALESCE(SUM(order_amount_cents), 0)
FROM payments
GROUP BY status, upper(currency)
ORDER BY status, upper(currency)`)
	if err != nil {
		return model.Overview{}, apperr.Internal(err)
	}
	for paymentRows.Next() {
		var p model.PaymentStatusTotal
		if err := paymentRows.Scan(&p.Status, &p.Currency, &p.Payments,
			&p.ChargedCents, &p.OriginalOrderCents); err != nil {
			paymentRows.Close()
			return model.Overview{}, apperr.Internal(err)
		}
		out.Payments.ByStatus = append(out.Payments.ByStatus, p)
	}
	if err := paymentRows.Err(); err != nil {
		paymentRows.Close()
		return model.Overview{}, apperr.Internal(err)
	}
	paymentRows.Close()

	limit := opts.TopItems
	if limit <= 0 {
		limit = 5
	}

	orderedRows, err := r.pool.Query(ctx, `
SELECT oi.product_id, oi.name, oi.sku, upper(o.currency),
       COALESCE(SUM(oi.qty), 0), COUNT(DISTINCT o.id),
       COALESCE(SUM(oi.unit_price_cents * oi.qty), 0)
FROM order_items oi JOIN orders o ON o.id = oi.order_id
WHERE o.status = 'paid'
GROUP BY oi.product_id, oi.name, oi.sku, upper(o.currency)
ORDER BY SUM(oi.qty) DESC, oi.product_id ASC
LIMIT $1`, limit)
	if err != nil {
		return model.Overview{}, apperr.Internal(err)
	}
	for orderedRows.Next() {
		var p model.OrderedProduct
		if err := orderedRows.Scan(&p.ProductID, &p.Name, &p.SKU, &p.Currency,
			&p.UnitsSold, &p.Orders, &p.RevenueCents); err != nil {
			orderedRows.Close()
			return model.Overview{}, apperr.Internal(err)
		}
		out.Demand.MostOrdered = append(out.Demand.MostOrdered, p)
	}
	if err := orderedRows.Err(); err != nil {
		orderedRows.Close()
		return model.Overview{}, apperr.Internal(err)
	}
	orderedRows.Close()

	wishRows, err := r.pool.Query(ctx, `
SELECT w.product_id, p.name, p.slug, COUNT(*)
FROM wishlist_items w JOIN products p ON p.id = w.product_id
GROUP BY w.product_id, p.name, p.slug
ORDER BY COUNT(*) DESC, w.product_id ASC
LIMIT $1`, limit)
	if err != nil {
		return model.Overview{}, apperr.Internal(err)
	}
	for wishRows.Next() {
		var p model.WishlistedProduct
		if err := wishRows.Scan(&p.ProductID, &p.Name, &p.Slug, &p.Wishlists); err != nil {
			wishRows.Close()
			return model.Overview{}, apperr.Internal(err)
		}
		out.Demand.MostWishlisted = append(out.Demand.MostWishlisted, p)
	}
	if err := wishRows.Err(); err != nil {
		wishRows.Close()
		return model.Overview{}, apperr.Internal(err)
	}
	wishRows.Close()

	cartRows, err := r.pool.Query(ctx, `
SELECT ci.product_id, p.name, p.slug, COALESCE(SUM(ci.qty), 0), COUNT(DISTINCT ci.cart_id)
FROM cart_items ci JOIN products p ON p.id = ci.product_id
GROUP BY ci.product_id, p.name, p.slug
ORDER BY SUM(ci.qty) DESC, ci.product_id ASC
LIMIT $1`, limit)
	if err != nil {
		return model.Overview{}, apperr.Internal(err)
	}
	for cartRows.Next() {
		var p model.CartedProduct
		if err := cartRows.Scan(&p.ProductID, &p.Name, &p.Slug, &p.CartQty, &p.Carts); err != nil {
			cartRows.Close()
			return model.Overview{}, apperr.Internal(err)
		}
		out.Demand.MostCarted = append(out.Demand.MostCarted, p)
	}
	if err := cartRows.Err(); err != nil {
		cartRows.Close()
		return model.Overview{}, apperr.Internal(err)
	}
	cartRows.Close()

	return out, nil
}
