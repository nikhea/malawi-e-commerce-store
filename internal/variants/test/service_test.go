package test

import (
	"context"
	"testing"

	"github.com/nikhea/malawi-e-commerce-store/internal/variants/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/variants/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/variants/service"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
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

func strptr(s string) *string { return &s }

func TestCreate(t *testing.T) {
	ctx := context.Background()

	t.Run("sku uppercased", func(t *testing.T) {
		svc := service.NewService(newFakeRepo())
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
		svc := service.NewService(newFakeRepo())
		_, err := svc.Create(ctx, public.CreateVariantInput{
			ProductID: "missing-product", Name: "X", SKU: "X-1", PriceCents: 100,
		})
		if apperr.CodeOf(err) != apperr.CodeNotFound {
			t.Fatalf("expected NOT_FOUND, got %v", err)
		}
	})

	t.Run("duplicate sku", func(t *testing.T) {
		svc := service.NewService(newFakeRepo())
		in := public.CreateVariantInput{ProductID: "p1", Name: "A", SKU: "DUP-1", PriceCents: 100}
		if _, err := svc.Create(ctx, in); err != nil {
			t.Fatalf("first: %v", err)
		}
		if _, err := svc.Create(ctx, in); apperr.CodeOf(err) != apperr.CodeConflict {
			t.Fatalf("expected CONFLICT, got %v", err)
		}
	})

	t.Run("negative price rejected", func(t *testing.T) {
		svc := service.NewService(newFakeRepo())
		_, err := svc.Create(ctx, public.CreateVariantInput{
			ProductID: "p1", Name: "A", SKU: "NEG-1", PriceCents: -5,
		})
		if apperr.CodeOf(err) != apperr.CodeValidation {
			t.Fatalf("expected VALIDATION_ERROR, got %v", err)
		}
	})
}

func TestUpdateImmutables(t *testing.T) {
	ctx := context.Background()
	svc := service.NewService(newFakeRepo())
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
