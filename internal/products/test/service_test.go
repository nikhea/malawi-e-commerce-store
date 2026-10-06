package test

import (
	"context"
	"testing"

	categoriespublic "github.com/nikhea/malawi-e-commerce-store/internal/categories/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/products/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/products/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/products/service"
	variantspublic "github.com/nikhea/malawi-e-commerce-store/internal/variants/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
)

// fakeRepo is an in-memory product store. No real DB.
type fakeRepo struct {
	byID   map[string]model.Product
	bySlug map[string]string
}

func newFakeRepo(seed ...model.Product) *fakeRepo {
	f := &fakeRepo{byID: map[string]model.Product{}, bySlug: map[string]string{}}
	for _, p := range seed {
		f.byID[p.ID] = p
		f.bySlug[p.Slug] = p.ID
	}
	return f
}

func (f *fakeRepo) Create(_ context.Context, p model.Product) (model.Product, error) {
	if _, taken := f.bySlug[p.Slug]; taken {
		return model.Product{}, apperr.Conflict("slug already in use")
	}
	p.ID = "prod-" + p.Slug
	p.IsActive = true
	f.byID[p.ID] = p
	f.bySlug[p.Slug] = p.ID
	return p, nil
}

func (f *fakeRepo) GetByID(_ context.Context, id string) (model.Product, error) {
	p, ok := f.byID[id]
	if !ok {
		return model.Product{}, apperr.NotFound("product not found")
	}
	return p, nil
}

func (f *fakeRepo) GetBySlug(_ context.Context, slug string) (model.Product, error) {
	id, ok := f.bySlug[slug]
	if !ok {
		return model.Product{}, apperr.NotFound("product not found")
	}
	return f.byID[id], nil
}

func (f *fakeRepo) List(_ context.Context, _ *string, _ bool, _, _ int) ([]model.Product, error) {
	out := []model.Product{}
	for _, p := range f.byID {
		out = append(out, p)
	}
	return out, nil
}

func (f *fakeRepo) Update(_ context.Context, p model.Product) (model.Product, error) {
	if _, ok := f.byID[p.ID]; !ok {
		return model.Product{}, apperr.NotFound("product not found")
	}
	f.byID[p.ID] = p
	return p, nil
}

func (f *fakeRepo) Delete(_ context.Context, id string) error {
	if _, ok := f.byID[id]; !ok {
		return apperr.NotFound("product not found")
	}
	delete(f.bySlug, f.byID[id].Slug)
	delete(f.byID, id)
	return nil
}

func (f *fakeRepo) SetImage(_ context.Context, id, url, _ string) (model.Product, error) {
	p, ok := f.byID[id]
	if !ok {
		return model.Product{}, apperr.NotFound("product not found")
	}
	p.ImageURL = url
	f.byID[id] = p
	return p, nil
}

// fakeCategories knows one category.
type fakeCategories struct{}

func (fakeCategories) Create(context.Context, categoriespublic.CreateCategoryInput) (categoriespublic.Category, error) {
	return categoriespublic.Category{}, nil
}
func (fakeCategories) GetByID(_ context.Context, id string) (categoriespublic.Category, error) {
	if id != "cat-1" {
		return categoriespublic.Category{}, apperr.NotFound("category not found")
	}
	return categoriespublic.Category{ID: "cat-1", Name: "Oil", Slug: "oil"}, nil
}
func (fakeCategories) GetBySlug(context.Context, string) (categoriespublic.Category, error) {
	return categoriespublic.Category{}, apperr.NotFound("category not found")
}
func (fakeCategories) List(context.Context) ([]categoriespublic.Category, error) { return nil, nil }
func (fakeCategories) Update(context.Context, string, categoriespublic.UpdateCategoryInput) (categoriespublic.Category, error) {
	return categoriespublic.Category{}, nil
}
func (fakeCategories) Delete(context.Context, string) error { return nil }
func (fakeCategories) SetImage(context.Context, string, string, string) (categoriespublic.Category, error) {
	return categoriespublic.Category{}, nil
}

// fakeVariants returns one SKU for product "prod-oil".
type fakeVariants struct{}

func (fakeVariants) Create(context.Context, variantspublic.CreateVariantInput) (variantspublic.Variant, error) {
	return variantspublic.Variant{}, nil
}
func (fakeVariants) ListByProduct(_ context.Context, productID string) ([]variantspublic.Variant, error) {
	if productID == "prod-oil" {
		return []variantspublic.Variant{{ID: "v1", ProductID: "prod-oil", Name: "500ml", SKU: "OIL-500", PriceCents: 2500, IsActive: true}}, nil
	}
	return []variantspublic.Variant{}, nil
}
func (fakeVariants) Update(context.Context, string, variantspublic.UpdateVariantInput) (variantspublic.Variant, error) {
	return variantspublic.Variant{}, nil
}
func (fakeVariants) Delete(context.Context, string) error { return nil }

func newService() public.Service {
	return service.NewService(newFakeRepo(), fakeCategories{}, fakeVariants{})
}

func TestMoneyAndCategory(t *testing.T) {
	ctx := context.Background()
	svc := newService()

	t.Run("price in minor units, category validated", func(t *testing.T) {
		p, err := svc.Create(ctx, public.CreateProductInput{
			CategoryID: strptr("cat-1"), Name: "Sunseed 2L", PriceCents: 12500,
		})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if p.PriceCents != 12500 || p.Currency != "MWK" || p.Slug != "sunseed-2l" {
			t.Fatalf("bad product: %+v", p)
		}
	})

	t.Run("unknown category rejected", func(t *testing.T) {
		_, err := svc.Create(ctx, public.CreateProductInput{
			CategoryID: strptr("nope"), Name: "X", PriceCents: 100,
		})
		if apperr.CodeOf(err) != apperr.CodeNotFound {
			t.Fatalf("expected NOT_FOUND, got %v", err)
		}
	})

	t.Run("negative price rejected", func(t *testing.T) {
		_, err := svc.Create(ctx, public.CreateProductInput{Name: "X", PriceCents: -1})
		if apperr.CodeOf(err) != apperr.CodeValidation {
			t.Fatalf("expected VALIDATION_ERROR, got %v", err)
		}
	})
}

func TestDetailComposesVariants(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo(model.Product{ID: "prod-oil", Name: "Oil", Slug: "oil", PriceCents: 100})
	svc := service.NewService(repo, fakeCategories{}, fakeVariants{})

	d, err := svc.GetByID(ctx, "prod-oil")
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if d.Product.ID != "prod-oil" || len(d.Variants) != 1 || d.Variants[0].SKU != "OIL-500" {
		t.Fatalf("bad detail: %+v", d)
	}
}

func strptr(s string) *string { return &s }
