package test

import (
	"context"
	"testing"
	"time"

	inventorypublic "github.com/nikhea/malawi-e-commerce-store/internal/inventory/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/variants/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/variants/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/variants/service"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	bulkdto "github.com/nikhea/malawi-e-commerce-store/pkg/bulk/dto"
)

// fakeRepo is an in-memory Repository. No real DB.
type fakeRepo struct {
	byID      map[string]model.Variant
	bySKU     map[string]string
	byProduct map[string][]string
}

func newFakeRepo(seed ...model.Variant) *fakeRepo {
	f := &fakeRepo{byID: map[string]model.Variant{}, bySKU: map[string]string{}, byProduct: map[string][]string{}}
	for _, v := range seed {
		f.byID[v.ID] = v
		f.bySKU[v.SKU] = v.ID
		f.byProduct[v.ProductID] = append(f.byProduct[v.ProductID], v.ID)
	}
	return f
}

func (f *fakeRepo) Create(_ context.Context, v model.Variant) (model.Variant, error) {
	if v.ProductID == "missing-product" {
		return model.Variant{}, apperr.NotFound("product not found")
	}
	if _, taken := f.bySKU[v.SKU]; taken {
		return model.Variant{}, apperr.Conflict("sku already in use")
	}
	v.ID = "var-" + v.SKU
	v.IsActive = true
	f.byID[v.ID] = v
	f.bySKU[v.SKU] = v.ID
	f.byProduct[v.ProductID] = append(f.byProduct[v.ProductID], v.ID)
	return v, nil
}

func (f *fakeRepo) GetByID(_ context.Context, id string) (model.Variant, error) {
	v, ok := f.byID[id]
	if !ok {
		return model.Variant{}, apperr.NotFound("variant not found")
	}
	return v, nil
}

func (f *fakeRepo) ListByProduct(_ context.Context, productID string) ([]model.Variant, error) {
	out := []model.Variant{}
	for _, id := range f.byProduct[productID] {
		out = append(out, f.byID[id])
	}
	return out, nil
}

func (f *fakeRepo) Update(_ context.Context, v model.Variant) (model.Variant, error) {
	if _, ok := f.byID[v.ID]; !ok {
		return model.Variant{}, apperr.NotFound("variant not found")
	}
	f.byID[v.ID] = v
	return v, nil
}

func (f *fakeRepo) Delete(_ context.Context, id string) error {
	v, ok := f.byID[id]
	if !ok {
		return apperr.NotFound("variant not found")
	}
	delete(f.bySKU, v.SKU)
	delete(f.byID, id)
	return nil
}

func (f *fakeRepo) ProductExists(_ context.Context, id string) (bool, error) {
	return id != "missing-product" && id != "", nil
}

func (f *fakeRepo) CreateMany(ctx context.Context, items []model.Variant) ([]model.Variant, int, error) {
	for i, v := range items {
		if v.ProductID == "missing-product" {
			return nil, i, apperr.NotFound("product not found")
		}
		if _, taken := f.bySKU[v.SKU]; taken {
			return nil, i, apperr.Conflict("sku already in use")
		}
	}
	out := make([]model.Variant, len(items))
	for i, v := range items {
		created, err := f.Create(ctx, v)
		if err != nil {
			return nil, i, err
		}
		out[i] = created
	}
	return out, -1, nil
}

func (f *fakeRepo) UpdateMany(ctx context.Context, items []model.Variant) ([]model.Variant, int, error) {
	for i, v := range items {
		if _, ok := f.byID[v.ID]; !ok {
			return nil, i, apperr.NotFound("variant not found")
		}
	}
	out := make([]model.Variant, len(items))
	for i, v := range items {
		updated, err := f.Update(ctx, v)
		if err != nil {
			return nil, i, err
		}
		out[i] = updated
	}
	return out, -1, nil
}

func strptr(s string) *string { return &s }

// fakeInventory reports 16 units for the stock test SKU.
type fakeInventory struct{}

func (fakeInventory) SetStock(context.Context, string, *string, int) (inventorypublic.Stock, error) {
	return inventorypublic.Stock{}, nil
}
func (fakeInventory) BulkSetStock(context.Context, []inventorypublic.BulkStockItem, bool) ([]bulkdto.BulkResult[inventorypublic.Stock], error) {
	return nil, nil
}
func (fakeInventory) GetStockByVariants(context.Context, []string) ([]inventorypublic.Stock, []string, error) {
	return nil, nil, nil
}
func (fakeInventory) GetStock(_ context.Context, productID string, variantID *string) (inventorypublic.Stock, error) {
	return inventorypublic.Stock{ProductID: productID, VariantID: variantID, OnHand: 16, Available: 16}, nil
}
func (fakeInventory) ListStockByProduct(_ context.Context, productID string) ([]inventorypublic.Stock, error) {
	id := "var-STOCK-1"
	return []inventorypublic.Stock{{ProductID: productID, VariantID: &id, OnHand: 16, Available: 16}}, nil
}
func (fakeInventory) Reserve(context.Context, string, []inventorypublic.ReserveLine, time.Duration) error {
	return nil
}
func (fakeInventory) ReleaseByOrder(context.Context, string) error { return nil }
func (fakeInventory) ConfirmByOrder(context.Context, string) error { return nil }
func (fakeInventory) ReleaseExpired(context.Context) ([]string, error) {
	return nil, nil
}

func newService(repo *fakeRepo) public.Service {
	return service.NewService(repo, fakeInventory{})
}

func TestCreate(t *testing.T) {
	ctx := context.Background()

	t.Run("sku uppercased", func(t *testing.T) {
		svc := newService(newFakeRepo())
		got, err := svc.Create(ctx, public.CreateVariantInput{
			ProductID: "p1", Name: "500ml", SKU: "oil-500-001", PriceCents: 2500,
		})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if got.SKU != "OIL-500-001" {
			t.Fatalf("expected uppercased sku, got %q", got.SKU)
		}
	})

	t.Run("missing product maps to not found", func(t *testing.T) {
		svc := newService(newFakeRepo())
		_, err := svc.Create(ctx, public.CreateVariantInput{
			ProductID: "missing-product", Name: "X", SKU: "X-1", PriceCents: 100,
		})
		if apperr.CodeOf(err) != apperr.CodeNotFound {
			t.Fatalf("expected NOT_FOUND, got %v", err)
		}
	})

	t.Run("duplicate sku", func(t *testing.T) {
		svc := newService(newFakeRepo())
		in := public.CreateVariantInput{ProductID: "p1", Name: "A", SKU: "DUP-1", PriceCents: 100}
		if _, err := svc.Create(ctx, in); err != nil {
			t.Fatalf("first: %v", err)
		}
		if _, err := svc.Create(ctx, in); apperr.CodeOf(err) != apperr.CodeConflict {
			t.Fatalf("expected CONFLICT, got %v", err)
		}
	})

	t.Run("negative price rejected", func(t *testing.T) {
		svc := newService(newFakeRepo())
		_, err := svc.Create(ctx, public.CreateVariantInput{
			ProductID: "p1", Name: "A", SKU: "NEG-1", PriceCents: -5,
		})
		if apperr.CodeOf(err) != apperr.CodeValidation {
			t.Fatalf("expected VALIDATION_ERROR, got %v", err)
		}
	})
}

func TestListByProductIncludesStock(t *testing.T) {
	ctx := context.Background()
	svc := newService(newFakeRepo())
	if _, err := svc.Create(ctx, public.CreateVariantInput{
		ProductID: "p1", Name: "L", SKU: "STOCK-1", PriceCents: 14500000,
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	variants, err := svc.ListByProduct(ctx, "p1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(variants) != 1 {
		t.Fatalf("expected 1 variant, got %d", len(variants))
	}
	if variants[0].Available != 16 || !variants[0].InStock {
		t.Fatalf("stock not attached: %+v", variants[0])
	}

	updated, err := svc.Update(ctx, variants[0].ID, public.UpdateVariantInput{Name: strptr("XL")})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Available != 16 || !updated.InStock {
		t.Fatalf("stock missing from update response: %+v", updated)
	}
}

func TestUpdateImmutables(t *testing.T) {
	ctx := context.Background()
	svc := newService(newFakeRepo())
	v, err := svc.Create(ctx, public.CreateVariantInput{
		ProductID: "p1", Name: "500ml", SKU: "IMM-1", PriceCents: 2500,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Price and name change; product and sku cannot (no fields for them).
	updated, err := svc.Update(ctx, v.ID, public.UpdateVariantInput{
		Name: strptr("1 Litre"), PriceCents: func() *int64 { p := int64(4800); return &p }(),
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "1 Litre" || updated.PriceCents != 4800 || updated.SKU != "IMM-1" {
		t.Fatalf("bad update: %+v", updated)
	}

	if _, err := svc.Update(ctx, "missing", public.UpdateVariantInput{}); apperr.CodeOf(err) != apperr.CodeNotFound {
		t.Fatalf("expected NOT_FOUND, got %v", err)
	}
}

func TestBulkCreate(t *testing.T) {
	ctx := context.Background()

	t.Run("missing product is whole request 404", func(t *testing.T) {
		svc := newService(newFakeRepo())
		_, err := svc.BulkCreate(ctx, "missing-product", []public.CreateVariantInput{
			{Name: "A", SKU: "A-1", PriceCents: 100},
		}, false, false)
		if apperr.CodeOf(err) != apperr.CodeNotFound {
			t.Fatalf("expected NOT_FOUND, got %v", err)
		}
	})

	t.Run("duplicate sku rejects the batch", func(t *testing.T) {
		svc := newService(newFakeRepo())
		_, err := svc.BulkCreate(ctx, "p1", []public.CreateVariantInput{
			{Name: "A", SKU: "dup-1", PriceCents: 100},
			{Name: "B", SKU: "DUP-1", PriceCents: 200},
		}, false, false)
		if apperr.CodeOf(err) != apperr.CodeDuplicateItem {
			t.Fatalf("expected DUPLICATE_ITEM, got %v", err)
		}
	})

	t.Run("existing sku is per item conflict", func(t *testing.T) {
		repo := newFakeRepo(model.Variant{ID: "existing", ProductID: "p1", Name: "Old", SKU: "DUP-1", IsActive: true})
		svc := newService(repo)
		results, err := svc.BulkCreate(ctx, "p1", []public.CreateVariantInput{
			{Name: "A", SKU: "DUP-1", PriceCents: 100},
			{Name: "B", SKU: "NEW-1", PriceCents: 200},
		}, false, false)
		if err != nil {
			t.Fatalf("bulk create: %v", err)
		}
		if results[0].Status != "error" || results[0].Error == nil || results[0].Error.Code != apperr.CodeConflict {
			t.Fatalf("expected per-item CONFLICT, got %+v", results[0])
		}
		if results[1].Status != "ok" || results[1].Data == nil || results[1].Data.SKU != "NEW-1" {
			t.Fatalf("expected second item ok, got %+v", results[1])
		}
	})

	t.Run("dry run projects without writing", func(t *testing.T) {
		repo := newFakeRepo()
		svc := newService(repo)
		results, err := svc.BulkCreate(ctx, "p1", []public.CreateVariantInput{
			{Name: "A", SKU: "dry-1", PriceCents: 100},
		}, false, true)
		if err != nil {
			t.Fatalf("dry run: %v", err)
		}
		if results[0].Status != "ok" || results[0].Data == nil || results[0].Data.SKU != "DRY-1" {
			t.Fatalf("bad dry run result: %+v", results[0])
		}
		if len(repo.byID) != 0 {
			t.Fatalf("dry run wrote rows: %d", len(repo.byID))
		}
	})

	t.Run("atomic rolls back on a bad item", func(t *testing.T) {
		repo := newFakeRepo()
		svc := newService(repo)
		results, err := svc.BulkCreate(ctx, "p1", []public.CreateVariantInput{
			{Name: "A", SKU: "OK-1", PriceCents: 100},
			{Name: "B", SKU: "BAD-1", PriceCents: -1},
		}, true, false)
		if err != nil {
			t.Fatalf("bulk create: %v", err)
		}
		for _, r := range results {
			if r.Status != "rolled_back" {
				t.Fatalf("expected rolled_back, got %+v", r)
			}
		}
		if results[1].Error == nil || results[1].Error.Code != apperr.CodeValidation {
			t.Fatalf("failing item should carry the error: %+v", results[1])
		}
		if len(repo.byID) != 0 {
			t.Fatalf("atomic rollback left rows: %d", len(repo.byID))
		}
	})

	t.Run("cap enforced", func(t *testing.T) {
		svc := newService(newFakeRepo())
		items := make([]public.CreateVariantInput, 51)
		for i := range items {
			items[i] = public.CreateVariantInput{Name: "A", SKU: "CAP-" + string(rune('A'+i)), PriceCents: 100}
		}
		if _, err := svc.BulkCreate(ctx, "p1", items, false, false); apperr.CodeOf(err) != apperr.CodeBulkLimitExceeded {
			t.Fatalf("expected BULK_LIMIT_EXCEEDED, got %v", err)
		}
	})
}

func TestBulkUpdate(t *testing.T) {
	ctx := context.Background()
	price := func(v int64) *int64 { return &v }

	seed := func(t *testing.T) (*fakeRepo, public.Service) {
		t.Helper()
		repo := newFakeRepo(
			model.Variant{ID: "v1", ProductID: "p1", Name: "A", SKU: "A-1", PriceCents: 100, IsActive: true},
			model.Variant{ID: "v2", ProductID: "p1", Name: "B", SKU: "B-1", PriceCents: 200, IsActive: true},
		)
		return repo, newService(repo)
	}

	t.Run("happy path returns availability", func(t *testing.T) {
		_, svc := seed(t)
		results, err := svc.BulkUpdate(ctx, []public.BulkUpdateItem{
			{ID: "v1", Patch: public.UpdateVariantInput{Name: strptr("AA")}},
			{ID: "v2", Patch: public.UpdateVariantInput{PriceCents: price(250)}},
		}, false, false)
		if err != nil {
			t.Fatalf("bulk update: %v", err)
		}
		if results[0].Data == nil || results[0].Data.Name != "AA" || results[0].Data.Available != 16 {
			t.Fatalf("bad first result: %+v", results[0])
		}
		if results[1].Data == nil || results[1].Data.PriceCents != 250 {
			t.Fatalf("bad second result: %+v", results[1])
		}
	})

	t.Run("empty patch is per item", func(t *testing.T) {
		_, svc := seed(t)
		results, err := svc.BulkUpdate(ctx, []public.BulkUpdateItem{
			{ID: "v1", Patch: public.UpdateVariantInput{}},
		}, false, false)
		if err != nil {
			t.Fatalf("bulk update: %v", err)
		}
		if results[0].Status != "error" || results[0].Error.Code != apperr.CodeValidation {
			t.Fatalf("expected VALIDATION_ERROR, got %+v", results[0])
		}
	})

	t.Run("duplicate ids reject the batch", func(t *testing.T) {
		_, svc := seed(t)
		_, err := svc.BulkUpdate(ctx, []public.BulkUpdateItem{
			{ID: "v1", Patch: public.UpdateVariantInput{Name: strptr("X")}},
			{ID: "v1", Patch: public.UpdateVariantInput{Name: strptr("Y")}},
		}, false, false)
		if apperr.CodeOf(err) != apperr.CodeDuplicateItem {
			t.Fatalf("expected DUPLICATE_ITEM, got %v", err)
		}
	})

	t.Run("dry run does not write", func(t *testing.T) {
		repo, svc := seed(t)
		results, err := svc.BulkUpdate(ctx, []public.BulkUpdateItem{
			{ID: "v1", Patch: public.UpdateVariantInput{Name: strptr("Projected")}},
		}, false, true)
		if err != nil {
			t.Fatalf("dry run: %v", err)
		}
		if results[0].Data == nil || results[0].Data.Name != "Projected" {
			t.Fatalf("bad dry run: %+v", results[0])
		}
		if repo.byID["v1"].Name != "A" {
			t.Fatalf("dry run wrote: %+v", repo.byID["v1"])
		}
	})

	t.Run("atomic rolls back when one id is missing", func(t *testing.T) {
		repo, svc := seed(t)
		results, err := svc.BulkUpdate(ctx, []public.BulkUpdateItem{
			{ID: "v1", Patch: public.UpdateVariantInput{Name: strptr("Changed")}},
			{ID: "missing", Patch: public.UpdateVariantInput{Name: strptr("Nope")}},
		}, true, false)
		if err != nil {
			t.Fatalf("bulk update: %v", err)
		}
		if results[0].Status != "rolled_back" || results[1].Status != "rolled_back" {
			t.Fatalf("expected rollback, got %+v", results)
		}
		if results[1].Error == nil || results[1].Error.Code != apperr.CodeNotFound {
			t.Fatalf("failing item should carry the error: %+v", results[1])
		}
		if repo.byID["v1"].Name != "A" {
			t.Fatalf("atomic rollback wrote: %+v", repo.byID["v1"])
		}
	})

	t.Run("cap enforced", func(t *testing.T) {
		_, svc := seed(t)
		items := make([]public.BulkUpdateItem, 51)
		if _, err := svc.BulkUpdate(ctx, items, false, false); apperr.CodeOf(err) != apperr.CodeBulkLimitExceeded {
			t.Fatalf("expected BULK_LIMIT_EXCEEDED, got %v", err)
		}
	})
}
