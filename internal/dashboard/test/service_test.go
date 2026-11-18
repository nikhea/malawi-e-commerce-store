package test

import (
	"context"
	"errors"
	"testing"

	"github.com/nikhea/malawi-e-commerce-store/internal/dashboard/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/dashboard/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/dashboard/service"
)

type fakeRepo struct {
	opts   model.OverviewOptions
	result model.Overview
	err    error
}

func (f *fakeRepo) Overview(_ context.Context, opts model.OverviewOptions) (model.Overview, error) {
	f.opts = opts
	return f.result, f.err
}

func TestOverviewMapsSnapshot(t *testing.T) {
	repo := &fakeRepo{result: model.Overview{
		Catalog: model.Catalog{TotalProducts: 12, ActiveProducts: 10},
		Customers: model.Customers{
			TotalCustomers: 30, PayingCustomers: 7, SucceededPaymentCustomers: 8,
		},
		Orders: model.Orders{
			Total: 9, Pending: 2, Paid: 6, Cancelled: 1,
			ByCurrency:            []model.CurrencyOrderTotal{{Currency: "USD", Orders: 9, TotalCents: 9000}},
			PaidRevenueByCurrency: []model.CurrencyOrderTotal{{Currency: "USD", Orders: 6, TotalCents: 6000}},
		},
		Fulfillment: model.Fulfillment{ActiveReservations: 2, ConfirmedReservations: 6, ExpiredActiveHolds: 1},
		Stock:       model.StockHealth{Rows: 12, OnHandQty: 100, ReservedQty: 8, AvailableQty: 92},
		Reviews:     model.Reviews{Total: 4, AverageRating: 4.5},
		Payments:    model.Payments{ByStatus: []model.PaymentStatusTotal{{Status: "succeeded", Currency: "USD", Payments: 6, ChargedCents: 6000}}},
		Demand: model.Demand{
			MostOrdered:    []model.OrderedProduct{{ProductID: "p1", UnitsSold: 5}},
			MostWishlisted: []model.WishlistedProduct{{ProductID: "p2", Wishlists: 3}},
			MostCarted:     []model.CartedProduct{{ProductID: "p3", CartQty: 2}},
		},
	}}
	svc := service.NewService(repo)

	got, err := svc.Overview(context.Background())
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if repo.opts.TopItems != public.TopItemsLimit {
		t.Fatalf("top items limit not passed: %+v", repo.opts)
	}
	if got.GeneratedAt.IsZero() {
		t.Fatal("generated_at missing")
	}
	if got.Catalog.TotalProducts != 12 || got.Customers.PayingCustomers != 7 {
		t.Fatalf("counters not mapped: %+v", got)
	}
	if got.Orders.Pending != 2 || got.Orders.Paid != 6 || got.Orders.Cancelled != 1 {
		t.Fatalf("order statuses not mapped: %+v", got.Orders)
	}
	if got.Fulfillment.ActiveReservations != 2 || got.Fulfillment.ExpiredActiveHolds != 1 {
		t.Fatalf("fulfillment not mapped: %+v", got.Fulfillment)
	}
	if got.Reviews.AverageRating != 4.5 || got.Payments.ByStatus[0].ChargedCents != 6000 {
		t.Fatalf("reviews/payments not mapped: %+v %+v", got.Reviews, got.Payments)
	}
	if len(got.Demand.MostOrdered) != 1 || len(got.Demand.MostWishlisted) != 1 || len(got.Demand.MostCarted) != 1 {
		t.Fatalf("demand rankings not mapped: %+v", got.Demand)
	}
}

func TestOverviewEmptySlicesAreNotNull(t *testing.T) {
	svc := service.NewService(&fakeRepo{})
	got, err := svc.Overview(context.Background())
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if got.Orders.ByCurrency == nil || got.Orders.PaidRevenueByCurrency == nil ||
		got.Payments.ByStatus == nil || got.Demand.MostOrdered == nil ||
		got.Demand.MostWishlisted == nil || got.Demand.MostCarted == nil {
		t.Fatalf("nil slice serializes as null: %+v", got)
	}
}

func TestOverviewPropagatesRepositoryError(t *testing.T) {
	want := errors.New("db down")
	svc := service.NewService(&fakeRepo{err: want})
	if _, err := svc.Overview(context.Background()); !errors.Is(err, want) {
		t.Fatalf("expected repository error, got %v", err)
	}
}
