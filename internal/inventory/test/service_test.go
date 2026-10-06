package test

import (
	"context"
	"testing"
	"time"

	"github.com/nikhea/malawi-e-commerce-store/internal/inventory/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/inventory/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/inventory/service"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
)

// fakeRepo mirrors the real semantics in memory: upsert, zero-stock for
// unknown SKUs, atomic reserve, idempotent settle.
type fakeRepo struct {
	stock map[string]*record
	holds map[string][]hold // orderRef → active holds
}

type record struct {
	stock model.Stock
}

type hold struct {
	productID string
	variantID *string
	qty       int
}

func key(productID string, variantID *string) string {
	if variantID == nil {
		return productID + "|"
	}
	return productID + "|" + *variantID
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{stock: map[string]*record{}, holds: map[string][]hold{}}
}

func (f *fakeRepo) UpsertStock(_ context.Context, productID string, variantID *string, qty int) (model.Stock, error) {
	s := model.Stock{ProductID: productID, VariantID: variantID, OnHand: qty}
	f.stock[key(productID, variantID)] = &record{stock: s}
	return s, nil
}

func (f *fakeRepo) GetStock(_ context.Context, productID string, variantID *string) (model.Stock, error) {
	if r, ok := f.stock[key(productID, variantID)]; ok {
		return r.stock, nil
	}
	return model.Stock{ProductID: productID, VariantID: variantID}, nil
}

func (f *fakeRepo) Reserve(_ context.Context, orderRef string, lines []model.Reservation, _ time.Time) error {
	for _, l := range lines {
		r := f.stock[key(l.ProductID, l.VariantID)]
		onHand := 0
		reserved := 0
		if r != nil {
			onHand, reserved = r.stock.OnHand, r.stock.Reserved
		}
		if onHand-reserved < l.Qty {
			return apperr.Conflict("insufficient stock")
		}
	}
	for _, l := range lines {
		k := key(l.ProductID, l.VariantID)
		if f.stock[k] == nil {
			f.stock[k] = &record{}
		}
		f.stock[k].stock.Reserved += l.Qty
		f.holds[orderRef] = append(f.holds[orderRef], hold{productID: l.ProductID, variantID: l.VariantID, qty: l.Qty})
	}
	return nil
}

func (f *fakeRepo) settle(orderRef string, confirm bool) error {
	for _, h := range f.holds[orderRef] {
		r := f.stock[key(h.productID, h.variantID)]
		r.stock.Reserved -= h.qty
		if confirm {
			r.stock.OnHand -= h.qty
		}
	}
	delete(f.holds, orderRef)
	return nil
}

func (f *fakeRepo) ReleaseByOrder(_ context.Context, orderRef string) error {
	return f.settle(orderRef, false)
}

func (f *fakeRepo) ConfirmByOrder(_ context.Context, orderRef string) error {
	return f.settle(orderRef, true)
}

func newService() public.Service {
	return service.NewService(newFakeRepo())
}

func TestReserveReleaseConfirm(t *testing.T) {
	ctx := context.Background()
	svc := newService()

	if _, err := svc.SetStock(ctx, "prod-1", nil, 10); err != nil {
		t.Fatalf("set stock: %v", err)
	}

	// Short line fails the whole order.
	err := svc.Reserve(ctx, "order-short", []public.ReserveLine{{ProductID: "prod-1", Qty: 99}}, time.Minute)
	if apperr.CodeOf(err) != apperr.CodeConflict {
		t.Fatalf("expected CONFLICT, got %v", err)
	}
	stock, _ := svc.GetStock(ctx, "prod-1", nil)
	if stock.Reserved != 0 {
		t.Fatalf("failed reserve leaked holds: %+v", stock)
	}

	// Successful reserve holds stock.
	err = svc.Reserve(ctx, "order-1", []public.ReserveLine{{ProductID: "prod-1", Qty: 4}}, time.Minute)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	stock, _ = svc.GetStock(ctx, "prod-1", nil)
	if stock.Available != 6 || stock.Reserved != 4 {
		t.Fatalf("bad hold: %+v", stock)
	}

	// Over-reserve against the hold fails.
	err = svc.Reserve(ctx, "order-2", []public.ReserveLine{{ProductID: "prod-1", Qty: 7}}, time.Minute)
	if apperr.CodeOf(err) != apperr.CodeConflict {
		t.Fatalf("expected CONFLICT, got %v", err)
	}

	// Release frees; confirm sells.
	if err := svc.ReleaseByOrder(ctx, "order-1"); err != nil {
		t.Fatalf("release: %v", err)
	}
	stock, _ = svc.GetStock(ctx, "prod-1", nil)
	if stock.Available != 10 {
		t.Fatalf("release didn't free: %+v", stock)
	}
	if err := svc.Reserve(ctx, "order-3", []public.ReserveLine{{ProductID: "prod-1", Qty: 4}}, time.Minute); err != nil {
		t.Fatalf("re-reserve: %v", err)
	}
	if err := svc.ConfirmByOrder(ctx, "order-3"); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	stock, _ = svc.GetStock(ctx, "prod-1", nil)
	if stock.OnHand != 6 || stock.Reserved != 0 || stock.Available != 6 {
		t.Fatalf("bad sale: %+v", stock)
	}

	// Idempotent: settling twice succeeds.
	if err := svc.ReleaseByOrder(ctx, "order-3"); err != nil {
		t.Fatalf("idempotent release: %v", err)
	}
	if err := svc.ConfirmByOrder(ctx, "ghost"); err != nil {
		t.Fatalf("idempotent confirm: %v", err)
	}
}

func TestValidation(t *testing.T) {
	ctx := context.Background()
	svc := newService()

	if _, err := svc.SetStock(ctx, "prod-1", nil, -1); apperr.CodeOf(err) != apperr.CodeValidation {
		t.Fatalf("negative stock: expected VALIDATION_ERROR, got %v", err)
	}
	if err := svc.Reserve(ctx, "", []public.ReserveLine{{ProductID: "p", Qty: 1}}, time.Minute); apperr.CodeOf(err) != apperr.CodeValidation {
		t.Fatalf("empty ref: expected VALIDATION_ERROR, got %v", err)
	}
	if err := svc.Reserve(ctx, "o", nil, time.Minute); apperr.CodeOf(err) != apperr.CodeValidation {
		t.Fatalf("no lines: expected VALIDATION_ERROR, got %v", err)
	}
	if err := svc.Reserve(ctx, "o", []public.ReserveLine{{ProductID: "p", Qty: 1}}, 0); apperr.CodeOf(err) != apperr.CodeValidation {
		t.Fatalf("no ttl: expected VALIDATION_ERROR, got %v", err)
	}
	stock, err := svc.GetStock(ctx, "untracked", nil)
	if err != nil || stock.Available != 0 {
		t.Fatalf("untracked sku should read zero: %v %+v", err, stock)
	}
}
