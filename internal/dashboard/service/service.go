// Package service owns the dashboard's read policy and model→wire mapping.
package service

import (
	"context"
	"time"

	"github.com/nikhea/malawi-e-commerce-store/internal/dashboard/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/dashboard/public"
)

var _ public.Service = (*service)(nil)

// Repository is the narrow persistence port the dashboard needs.
type Repository interface {
	Overview(ctx context.Context, opts model.OverviewOptions) (model.Overview, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) public.Service {
	return &service{repo: repo}
}

// Overview maps the module-private snapshot to the wire shape. All slices
// are non-nil so empty results serialize as [] rather than null.
func (s *service) Overview(ctx context.Context) (public.Overview, error) {
	in, err := s.repo.Overview(ctx, model.OverviewOptions{TopItems: public.TopItemsLimit})
	if err != nil {
		return public.Overview{}, err
	}
	return public.Overview{
		GeneratedAt: time.Now().UTC(),
		Catalog: public.Catalog{
			TotalProducts: in.Catalog.TotalProducts, ActiveProducts: in.Catalog.ActiveProducts,
			InactiveProducts: in.Catalog.InactiveProducts, TotalVariants: in.Catalog.TotalVariants,
			ActiveVariants: in.Catalog.ActiveVariants, TotalCategories: in.Catalog.TotalCategories,
			ReadyImages: in.Catalog.ReadyImages,
		},
		Customers: public.Customers{
			TotalUsers: in.Customers.TotalUsers, TotalCustomers: in.Customers.TotalCustomers,
			VerifiedCustomers: in.Customers.VerifiedCustomers, TotalAdmins: in.Customers.TotalAdmins,
			NewCustomers30D: in.Customers.NewCustomers30D, PayingCustomers: in.Customers.PayingCustomers,
			SucceededPaymentCustomers: in.Customers.SucceededPaymentCustomers,
		},
		Orders: public.Orders{
			Total: in.Orders.Total, Pending: in.Orders.Pending, Paid: in.Orders.Paid,
			Cancelled: in.Orders.Cancelled, Last30D: in.Orders.Last30D,
			ByCurrency:            currencyTotals(in.Orders.ByCurrency),
			PaidRevenueByCurrency: currencyTotals(in.Orders.PaidRevenueByCurrency),
		},
		Fulfillment: public.Fulfillment{
			ActiveReservations:    in.Fulfillment.ActiveReservations,
			ConfirmedReservations: in.Fulfillment.ConfirmedReservations,
			ReleasedReservations:  in.Fulfillment.ReleasedReservations,
			ActiveReservedQty:     in.Fulfillment.ActiveReservedQty,
			ConfirmedQty:          in.Fulfillment.ConfirmedQty,
			ReleasedQty:           in.Fulfillment.ReleasedQty,
			ExpiredActiveHolds:    in.Fulfillment.ExpiredActiveHolds,
		},
		Stock: public.StockHealth{
			Rows: in.Stock.Rows, OnHandQty: in.Stock.OnHandQty,
			ReservedQty: in.Stock.ReservedQty, AvailableQty: in.Stock.AvailableQty,
			OutOfStockRows: in.Stock.OutOfStockRows,
		},
		Reviews: public.Reviews{
			Total: in.Reviews.Total, AverageRating: in.Reviews.AverageRating,
			Last30D: in.Reviews.Last30D,
		},
		Payments: public.Payments{
			ByStatus: paymentTotals(in.Payments.ByStatus),
		},
		Demand: public.Demand{
			MostOrdered:    orderedProducts(in.Demand.MostOrdered),
			MostWishlisted: wishlistedProducts(in.Demand.MostWishlisted),
			MostCarted:     cartedProducts(in.Demand.MostCarted),
		},
	}, nil
}

func currencyTotals(in []model.CurrencyOrderTotal) []public.CurrencyOrderTotal {
	out := make([]public.CurrencyOrderTotal, 0, len(in))
	for _, t := range in {
		out = append(out, public.CurrencyOrderTotal{
			Currency: t.Currency, Orders: t.Orders, TotalCents: t.TotalCents,
		})
	}
	return out
}

func paymentTotals(in []model.PaymentStatusTotal) []public.PaymentStatusTotal {
	out := make([]public.PaymentStatusTotal, 0, len(in))
	for _, p := range in {
		out = append(out, public.PaymentStatusTotal{
			Status: p.Status, Currency: p.Currency, Payments: p.Payments,
			ChargedCents: p.ChargedCents, OriginalOrderCents: p.OriginalOrderCents,
		})
	}
	return out
}

func orderedProducts(in []model.OrderedProduct) []public.OrderedProduct {
	out := make([]public.OrderedProduct, 0, len(in))
	for _, p := range in {
		out = append(out, public.OrderedProduct{
			ProductID: p.ProductID, Name: p.Name, SKU: p.SKU, Currency: p.Currency,
			UnitsSold: p.UnitsSold, Orders: p.Orders, RevenueCents: p.RevenueCents,
		})
	}
	return out
}

func wishlistedProducts(in []model.WishlistedProduct) []public.WishlistedProduct {
	out := make([]public.WishlistedProduct, 0, len(in))
	for _, p := range in {
		out = append(out, public.WishlistedProduct{
			ProductID: p.ProductID, Name: p.Name, Slug: p.Slug, Wishlists: p.Wishlists,
		})
	}
	return out
}

func cartedProducts(in []model.CartedProduct) []public.CartedProduct {
	out := make([]public.CartedProduct, 0, len(in))
	for _, p := range in {
		out = append(out, public.CartedProduct{
			ProductID: p.ProductID, Name: p.Name, Slug: p.Slug,
			CartQty: p.CartQty, Carts: p.Carts,
		})
	}
	return out
}
