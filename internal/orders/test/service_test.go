package test

import (
	"context"
	"testing"
	"time"

	cartpublic "github.com/nikhea/malawi-e-commerce-store/internal/cart/public"
	inventorypublic "github.com/nikhea/malawi-e-commerce-store/internal/inventory/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/orders/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/orders/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/orders/service"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	"github.com/nikhea/malawi-e-commerce-store/pkg/events"
)

// fakeRepo is an in-memory orders store with guarded transitions.
type fakeRepo struct {
	orders map[string]model.Order
	items  map[string][]model.Item
	seq    int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{orders: map[string]model.Order{}, items: map[string][]model.Item{}}
}

func (f *fakeRepo) Create(_ context.Context, o model.Order, items []model.Item) (model.Order, error) {
	f.seq++
	o.ID = "order-n"
	o.Status = public.StatusPending
	f.orders[o.ID] = o
	f.items[o.ID] = items
	return o, nil
}

func (f *fakeRepo) GetByID(_ context.Context, id string) (model.Order, error) {
	o, ok := f.orders[id]
	if !ok {
		return model.Order{}, apperr.NotFound("order not found")
	}
	return o, nil
}

func (f *fakeRepo) ListByUser(_ context.Context, userID string) ([]model.Order, error) {
	out := []model.Order{}
	for _, o := range f.orders {
		if o.UserID == userID {
			out = append(out, o)
		}
	}
	return out, nil
}

func (f *fakeRepo) transition(id, status string) (model.Order, error) {
	o, ok := f.orders[id]
	if !ok {
		return model.Order{}, apperr.NotFound("order not found")
	}
	if o.Status != public.StatusPending {
		return model.Order{}, apperr.Conflict("order is not pending")
	}
	o.Status = status
	f.orders[id] = o
	return o, nil
}

func (f *fakeRepo) MarkPaid(_ context.Context, id string) (model.Order, error) {
	return f.transition(id, public.StatusPaid)
}

func (f *fakeRepo) Cancel(_ context.Context, id string) (model.Order, error) {
	return f.transition(id, public.StatusCancelled)
}

func (f *fakeRepo) GetItems(_ context.Context, id string) ([]model.Item, error) {
	return f.items[id], nil
}

// fakeCart serves one stocked cart for "u1".
type fakeCart struct {
	empty      bool
	cleared    bool
	reserveErr error
}

func (f *fakeCart) Get(_ context.Context, userID string) (cartpublic.Cart, error) {
	if userID != "u1" {
		return cartpublic.Cart{}, apperr.NotFound("user not found")
	}
	if f.empty {
		return cartpublic.Cart{ID: "cart-1", UserID: "u1", Items: []cartpublic.Line{}}, nil
	}
	return cartpublic.Cart{ID: "cart-1", UserID: "u1", Currency: "MWK", SubtotalCents: 2000, Items: []cartpublic.Line{
		{ID: "l1", ProductID: "p1", Name: "Oil", UnitPriceCents: 1000, Currency: "MWK", Qty: 2, LineTotalCents: 2000},
	}}, nil
}

func (f *fakeCart) AddItem(context.Context, cartpublic.AddItemInput) (cartpublic.Cart, error) {
	return cartpublic.Cart{}, nil
}
func (f *fakeCart) SetQty(context.Context, string, string, int) (cartpublic.Cart, error) {
	return cartpublic.Cart{}, nil
}
func (f *fakeCart) RemoveItem(context.Context, string, string) (cartpublic.Cart, error) {
	return cartpublic.Cart{}, nil
}
func (f *fakeCart) Clear(_ context.Context, userID string) error {
	f.cleared = true
	return nil
}

// fakeInventory records calls; reserve fails when told to.
type fakeInventory struct {
	reserved    []string
	released    []string
	confirmed   []string
	failReserve bool
}

func (f *fakeInventory) SetStock(context.Context, string, *string, int) (inventorypublic.Stock, error) {
	return inventorypublic.Stock{}, nil
}
func (f *fakeInventory) GetStock(context.Context, string, *string) (inventorypublic.Stock, error) {
	return inventorypublic.Stock{}, nil
}
func (f *fakeInventory) Reserve(_ context.Context, orderRef string, _ []inventorypublic.ReserveLine, _ time.Duration) error {
	if f.failReserve {
		return apperr.Conflict("insufficient stock")
	}
	f.reserved = append(f.reserved, orderRef)
	return nil
}
func (f *fakeInventory) ReleaseByOrder(_ context.Context, orderRef string) error {
	f.released = append(f.released, orderRef)
	return nil
}
func (f *fakeInventory) ConfirmByOrder(_ context.Context, orderRef string) error {
	f.confirmed = append(f.confirmed, orderRef)
	return nil
}

type fixture struct {
	svc       public.Service
	cart      *fakeCart
	inventory *fakeInventory
	published []string
}

func newFixture() *fixture {
	fx := &fixture{cart: &fakeCart{}, inventory: &fakeInventory{}}
	bus := events.New()
	bus.Subscribe(public.OrderCreated, func(_ context.Context, e events.Event) { fx.published = append(fx.published, e.Name) })
	bus.Subscribe(public.OrderPaid, func(_ context.Context, e events.Event) { fx.published = append(fx.published, e.Name) })
	bus.Subscribe(public.OrderCancelled, func(_ context.Context, e events.Event) { fx.published = append(fx.published, e.Name) })
	fx.svc = service.NewService(newFakeRepo(), fx.cart, fx.inventory, bus, service.Config{})
	return fx
}

func TestCheckout(t *testing.T) {
	ctx := context.Background()

	t.Run("freezes cart, reserves, clears, publishes", func(t *testing.T) {
		fx := newFixture()
		order, err := fx.svc.Checkout(ctx, "u1")
		if err != nil {
			t.Fatalf("checkout: %v", err)
		}
		if order.Status != public.StatusPending || order.SubtotalCents != 2000 || len(order.Lines) != 1 {
			t.Fatalf("bad order: %+v", order)
		}
		if len(fx.inventory.reserved) != 1 || !fx.cart.cleared {
			t.Fatalf("side effects missing: %+v %+v", fx.inventory.reserved, fx.cart.cleared)
		}
		if len(fx.published) != 1 || fx.published[0] != public.OrderCreated {
			t.Fatalf("events wrong: %v", fx.published)
		}
	})

	t.Run("empty cart rejected", func(t *testing.T) {
		fx := newFixture()
		fx.cart.empty = true
		if _, err := fx.svc.Checkout(ctx, "u1"); apperr.CodeOf(err) != apperr.CodeValidation {
			t.Fatalf("expected VALIDATION_ERROR, got %v", err)
		}
	})

	t.Run("reserve failure compensates", func(t *testing.T) {
		fx := newFixture()
		fx.inventory.failReserve = true
		if _, err := fx.svc.Checkout(ctx, "u1"); apperr.CodeOf(err) != apperr.CodeConflict {
			t.Fatalf("expected CONFLICT, got %v", err)
		}
		if fx.cart.cleared {
			t.Fatal("cart cleared despite failed reserve")
		}
		orders, _ := fx.svc.ListMine(ctx, "u1")
		for _, o := range orders {
			if o.Status == public.StatusPending {
				t.Fatalf("orphan pending order: %+v", o)
			}
		}
	})
}

func TestPayCancel(t *testing.T) {
	ctx := context.Background()
	fx := newFixture()
	order, err := fx.svc.Checkout(ctx, "u1")
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}

	// Owner isolation.
	if _, err := fx.svc.GetByID(ctx, "intruder", order.ID); apperr.CodeOf(err) != apperr.CodeNotFound {
		t.Fatalf("expected NOT_FOUND for intruder, got %v", err)
	}

	// Pay: confirms stock, publishes.
	paid, err := fx.svc.MarkPaid(ctx, order.ID)
	if err != nil || paid.Status != public.StatusPaid {
		t.Fatalf("pay: %v %+v", err, paid)
	}
	if len(fx.inventory.confirmed) != 1 || fx.published[len(fx.published)-1] != public.OrderPaid {
		t.Fatalf("pay side effects wrong: %+v %v", fx.inventory.confirmed, fx.published)
	}
	// Repeat pay succeeds (idempotent).
	if _, err := fx.svc.MarkPaid(ctx, order.ID); err != nil {
		t.Fatalf("repeat pay: %v", err)
	}
	// Paid order can't cancel.
	if _, err := fx.svc.Cancel(ctx, "u1", order.ID); apperr.CodeOf(err) != apperr.CodeConflict {
		t.Fatalf("expected CONFLICT, got %v", err)
	}

	// Cancel path on a fresh order.
	order2, err := fx.svc.Checkout(ctx, "u1")
	if err != nil {
		t.Fatalf("checkout 2: %v", err)
	}
	cancelled, err := fx.svc.Cancel(ctx, "u1", order2.ID)
	if err != nil || cancelled.Status != public.StatusCancelled {
		t.Fatalf("cancel: %v %+v", err, cancelled)
	}
	if len(fx.inventory.released) != 1 {
		t.Fatalf("release missing: %+v", fx.inventory.released)
	}
}
