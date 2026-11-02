package test

import (
	"context"
	"errors"
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
	stock     map[string]*record
	holds     map[string][]hold // orderRef → active holds
	known     map[string]string // variant id → product id
	bulkErrAt int               // index that fails in an atomic bulk
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
	return &fakeRepo{
		stock: map[string]*record{}, holds: map[string][]hold{},
		known: map[string]string{}, bulkErrAt: -1,
	}
}

func (f *fakeRepo) cloneStock() map[string]*record {
	out := make(map[string]*record, len(f.stock))
	for k, v := range f.stock {
		cp := *v
		out[k] = &cp
	}
	return out
}

func (f *fakeRepo) UpsertStock(_ context.Context, productID string, variantID *string, qty int) (model.Stock, error) {
	if productID == "missing-product" {
		return model.Stock{}, apperr.NotFound("product not found")
	}
	if variantID != nil {
		if _, ok := f.known[*variantID]; !ok {
			return model.Stock{}, apperr.NotFound("variant not found")
		}
	}
	s := model.Stock{ProductID: productID, VariantID: variantID, OnHand: qty}
	f.stock[key(productID, variantID)] = &record{stock: s}
	return s, nil
}

// BulkUpsertStock mirrors the repository contract: atomic restores the map
// to its pre-call snapshot on the first failure.
func (f *fakeRepo) BulkUpsertStock(ctx context.Context, items []model.StockInput, atomic bool) ([]model.Stock, []error, error) {
	errs := make([]error, len(items))
	out := make([]model.Stock, len(items))
	if !atomic {
		for i, it := range items {
			out[i], errs[i] = f.UpsertStock(ctx, it.ProductID, it.VariantID, it.Qty)
		}
		return out, errs, nil
	}
	snapshot := f.cloneStock()
	for i, it := range items {
		if i == f.bulkErrAt {
			errs[i] = apperr.Internal(errors.New("bulk write failed"))
			f.stock = snapshot
			return nil, errs, nil
		}
		stock, err := f.UpsertStock(ctx, it.ProductID, it.VariantID, it.Qty)
		if err != nil {
			errs[i] = err
			f.stock = snapshot
			return nil, errs, nil
		}
		out[i] = stock
	}
	return out, errs, nil
}

func (f *fakeRepo) ListStockByVariantIDs(_ context.Context, variantIDs []string) ([]model.Stock, error) {
	out := []model.Stock{}
	for _, id := range variantIDs {
		variantID := id
		productID, ok := f.known[variantID]
		if !ok {
			continue
		}
		if r := f.stock[key(productID, &variantID)]; r != nil {
			out = append(out, r.stock)
			continue
		}
		out = append(out, model.Stock{ProductID: productID, VariantID: &variantID})
	}
	return out, nil
}

func (f *fakeRepo) GetStock(_ context.Context, productID string, variantID *string) (model.Stock, error) {
	if r, ok := f.stock[key(productID, variantID)]; ok {
		return r.stock, nil
	}
	return model.Stock{ProductID: productID, VariantID: variantID}, nil
}

func (f *fakeRepo) ListStockByProduct(_ context.Context, productID string) ([]model.Stock, error) {
	out := []model.Stock{}
	for _, r := range f.stock {
		if r.stock.ProductID == productID {
			out = append(out, r.stock)
		}
	}
	return out, nil
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

func (f *fakeRepo) ReleaseExpired(_ context.Context) ([]string, error) {
	return nil, nil
}

func newService() public.Service {
	return service.NewService(newFakeRepo())
}

func newServiceRepo() (public.Service, *fakeRepo) {
	repo := newFakeRepo()
	return service.NewService(repo), repo
}

func strptr(s string) *string { return &s }

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

func TestListStockByProduct(t *testing.T) {
	ctx := context.Background()
	svc, repo := newServiceRepo()
	variantID := "var-1"
	repo.known[variantID] = "prod-1"

	if _, err := svc.SetStock(ctx, "prod-1", nil, 4); err != nil {
		t.Fatalf("base stock: %v", err)
	}
	if _, err := svc.SetStock(ctx, "prod-1", &variantID, 9); err != nil {
		t.Fatalf("variant stock: %v", err)
	}

	rows, err := svc.ListStockByProduct(ctx, "prod-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d: %+v", len(rows), rows)
	}
	if _, err := svc.ListStockByProduct(ctx, "  "); apperr.CodeOf(err) != apperr.CodeValidation {
		t.Fatalf("empty product: expected VALIDATION_ERROR, got %v", err)
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

func TestBulkSetStockNonAtomicPartialFailure(t *testing.T) {
	ctx := context.Background()
	svc, repo := newServiceRepo()
	variantID := "var-1"
	repo.known[variantID] = "prod-1"

	results, err := svc.BulkSetStock(ctx, []public.BulkStockItem{
		{ProductID: "prod-1", Qty: 5},
		{ProductID: "prod-1", VariantID: &variantID, Qty: 9},
		{ProductID: "prod-1", VariantID: strptr("ghost"), Qty: 3},
	}, false)
	if err != nil {
		t.Fatalf("non-atomic bulk: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Status != "ok" || results[1].Status != "ok" {
		t.Fatalf("expected first two ok: %+v", results)
	}
	if results[2].Status != "error" || results[2].Error == nil || results[2].Error.Code != apperr.CodeNotFound {
		t.Fatalf("expected per-item NOT_FOUND: %+v", results[2])
	}

	base, _ := svc.GetStock(ctx, "prod-1", nil)
	if base.OnHand != 5 {
		t.Fatalf("base line not written: %+v", base)
	}
	variant, _ := svc.GetStock(ctx, "prod-1", &variantID)
	if variant.OnHand != 9 {
		t.Fatalf("variant line not written: %+v", variant)
	}
}

func TestBulkSetStockAtomicRollsBackValidation(t *testing.T) {
	ctx := context.Background()
	svc, repo := newServiceRepo()

	results, err := svc.BulkSetStock(ctx, []public.BulkStockItem{
		{ProductID: "prod-1", Qty: 5},
		{ProductID: "prod-2", Qty: -1},
	}, true)
	if err != nil {
		t.Fatalf("atomic bulk: %v", err)
	}
	for _, r := range results {
		if r.Status != "rolled_back" {
			t.Fatalf("expected all rolled_back: %+v", results)
		}
	}
	if results[1].Error == nil || results[1].Error.Code != apperr.CodeValidation {
		t.Fatalf("failing item lost its error: %+v", results[1])
	}
	if len(repo.stock) != 0 {
		t.Fatalf("atomic validation failure wrote rows: %+v", repo.stock)
	}
}

func TestBulkSetStockAtomicRollsBackRepoFailure(t *testing.T) {
	ctx := context.Background()
	svc, repo := newServiceRepo()
	repo.bulkErrAt = 1

	results, err := svc.BulkSetStock(ctx, []public.BulkStockItem{
		{ProductID: "prod-1", Qty: 5},
		{ProductID: "prod-2", Qty: 7},
	}, true)
	if err != nil {
		t.Fatalf("atomic bulk: %v", err)
	}
	for _, r := range results {
		if r.Status != "rolled_back" {
			t.Fatalf("expected all rolled_back: %+v", results)
		}
	}
	if len(repo.stock) != 0 {
		t.Fatalf("repo failure leaked the first write: %+v", repo.stock)
	}
}

func TestBulkSetStockCapsAndDuplicates(t *testing.T) {
	ctx := context.Background()
	svc := newService()

	tooMany := make([]public.BulkStockItem, 51)
	for i := range tooMany {
		tooMany[i] = public.BulkStockItem{ProductID: "prod-1", Qty: 1}
	}
	if _, err := svc.BulkSetStock(ctx, tooMany, false); apperr.CodeOf(err) != apperr.CodeBulkLimitExceeded {
		t.Fatalf("expected BULK_LIMIT_EXCEEDED, got %v", err)
	}

	if _, err := svc.BulkSetStock(ctx, []public.BulkStockItem{
		{ProductID: "prod-1", Qty: 1},
		{ProductID: "prod-1", Qty: 2},
	}, false); apperr.CodeOf(err) != apperr.CodeDuplicateItem {
		t.Fatalf("expected DUPLICATE_ITEM, got %v", err)
	}

	if _, err := svc.BulkSetStock(ctx, nil, false); apperr.CodeOf(err) != apperr.CodeValidation {
		t.Fatalf("empty batch: expected VALIDATION_ERROR, got %v", err)
	}
}

func TestGetStockByVariants(t *testing.T) {
	ctx := context.Background()
	svc, repo := newServiceRepo()
	known := "var-1"
	untracked := "var-2"
	repo.known[known] = "prod-1"
	repo.known[untracked] = "prod-1"

	if _, err := svc.SetStock(ctx, "prod-1", &known, 12); err != nil {
		t.Fatalf("set stock: %v", err)
	}

	rows, notFound, err := svc.GetStockByVariants(ctx, []string{known, untracked, "ghost"})
	if err != nil {
		t.Fatalf("get by variants: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d: %+v", len(rows), rows)
	}
	if rows[0].OnHand != 12 || rows[0].Available != 12 {
		t.Fatalf("bad stocked row: %+v", rows[0])
	}
	if rows[1].OnHand != 0 || rows[1].Available != 0 {
		t.Fatalf("known untracked variant should read zero: %+v", rows[1])
	}
	if len(notFound) != 1 || notFound[0] != "ghost" {
		t.Fatalf("expected ghost in notFound, got %v", notFound)
	}

	if _, _, err := svc.GetStockByVariants(ctx, []string{known, known}); apperr.CodeOf(err) != apperr.CodeDuplicateItem {
		t.Fatalf("expected DUPLICATE_ITEM, got %v", err)
	}
	if _, _, err := svc.GetStockByVariants(ctx, nil); apperr.CodeOf(err) != apperr.CodeValidation {
		t.Fatalf("empty ids: expected VALIDATION_ERROR, got %v", err)
	}
}
