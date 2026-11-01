package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	categoriespublic "github.com/nikhea/malawi-e-commerce-store/internal/categories/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/products/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/products/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/products/service"
	variantspublic "github.com/nikhea/malawi-e-commerce-store/internal/variants/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
)

// fakeRepo is an in-memory product store. No real DB.
type fakeRepo struct {
	byID       map[string]model.Product
	bySlug     map[string]string
	lastFilter model.ListFilter
	images     map[string][]model.Image
	nextImage  int
}

func newFakeRepo(seed ...model.Product) *fakeRepo {
	f := &fakeRepo{
		byID:   map[string]model.Product{},
		bySlug: map[string]string{},
		images: map[string][]model.Image{},
	}
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

func (f *fakeRepo) List(_ context.Context, filter model.ListFilter) ([]model.Product, int64, error) {
	f.lastFilter = filter
	out := []model.Product{}
	for _, p := range f.byID {
		out = append(out, p)
	}
	return out, int64(len(out)), nil
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

// promoteFirst mirrors the repository's "lowest-position ready image"
// rule for the in-memory store.
func (f *fakeRepo) promoteFirst(productID string) {
	p := f.byID[productID]
	p.ImageURL, p.ImagePublicID = "", ""
	for _, img := range f.images[productID] {
		if img.URL != "" {
			p.ImageURL, p.ImagePublicID = img.URL, img.PublicID
			break
		}
	}
	f.byID[productID] = p
}

func (f *fakeRepo) CreateImage(_ context.Context, productID string) (model.Image, error) {
	if _, ok := f.byID[productID]; !ok {
		return model.Image{}, apperr.NotFound("product not found")
	}
	f.nextImage++
	img := model.Image{
		ID:        fmt.Sprintf("img-%d", f.nextImage),
		ProductID: productID,
		Position:  len(f.images[productID]),
		CreatedAt: time.Now(),
	}
	f.images[productID] = append(f.images[productID], img)
	return img, nil
}

func (f *fakeRepo) ListImages(_ context.Context, productID string) ([]model.Image, error) {
	return append([]model.Image{}, f.images[productID]...), nil
}

func (f *fakeRepo) GetImage(_ context.Context, productID, imageID string) (model.Image, error) {
	for _, img := range f.images[productID] {
		if img.ID == imageID {
			return img, nil
		}
	}
	return model.Image{}, apperr.NotFound("image not found")
}

func (f *fakeRepo) DeleteImage(_ context.Context, productID, imageID string) error {
	list := f.images[productID]
	for idx, img := range list {
		if img.ID != imageID {
			continue
		}
		f.images[productID] = append(append([]model.Image{}, list[:idx]...), list[idx+1:]...)
		p := f.byID[productID]
		if img.URL != "" && ((img.PublicID != "" && p.ImagePublicID == img.PublicID) || p.ImageURL == img.URL) {
			f.promoteFirst(productID)
		}
		return nil
	}
	return apperr.NotFound("image not found")
}

func (f *fakeRepo) SetPrimaryImage(_ context.Context, productID, imageID string) (model.Product, error) {
	for _, img := range f.images[productID] {
		if img.ID != imageID {
			continue
		}
		p := f.byID[productID]
		p.ImageURL, p.ImagePublicID = img.URL, img.PublicID
		f.byID[productID] = p
		return p, nil
	}
	return model.Product{}, apperr.NotFound("image not found")
}

func (f *fakeRepo) CompleteImage(_ context.Context, imageID, url, publicID string) (model.Image, error) {
	for productID, list := range f.images {
		for idx, img := range list {
			if img.ID != imageID {
				continue
			}
			img.URL, img.PublicID = url, publicID
			f.images[productID][idx] = img
			return img, nil
		}
	}
	return model.Image{}, nil // row deleted mid-upload
}

func (f *fakeRepo) PromoteIfEmpty(_ context.Context, productID string) (bool, error) {
	if f.byID[productID].ImageURL != "" {
		return false, nil
	}
	f.promoteFirst(productID)
	return true, nil
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
func (fakeCategories) List(context.Context, categoriespublic.ListFilter) (categoriespublic.Page, error) {
	return categoriespublic.Page{}, nil
}
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
		return []variantspublic.Variant{{
			ID: "v1", ProductID: "prod-oil", Name: "500ml", SKU: "OIL-500",
			PriceCents: 2500, IsActive: true, Available: 16, InStock: true,
		}}, nil
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
		if p.PriceCents != 12500 || p.Currency != "USD" || p.Slug != "sunseed-2l" {
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
	if d.Variants[0].Available != 16 || !d.Variants[0].InStock {
		t.Fatalf("variant stock not composed: %+v", d.Variants[0])
	}
}

func strptr(s string) *string   { return &s }
func i64ptr(v int64) *int64     { return &v }
func f64ptr(v float64) *float64 { return &v }

func TestListPaginationAndFilters(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo(
		model.Product{ID: "p1", Name: "Alpha", Slug: "alpha", PriceCents: 100},
		model.Product{ID: "p2", Name: "Beta", Slug: "beta", PriceCents: 200},
	)
	svc := service.NewService(repo, fakeCategories{}, fakeVariants{})

	t.Run("defaults to first page of 20", func(t *testing.T) {
		page, err := svc.List(ctx, public.ListFilter{ActiveOnly: true})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if page.Meta.Page != 1 || page.Meta.Limit != 12 || page.Meta.Offset != 0 || page.Meta.Total != 2 {
			t.Fatalf("bad meta: %+v", page.Meta)
		}
		if page.Meta.TotalPages != 1 || page.Meta.HasNext || page.Meta.HasPrev {
			t.Fatalf("bad flags: %+v", page.Meta)
		}
		if repo.lastFilter.Sort != "name_asc" || !repo.lastFilter.ActiveOnly {
			t.Fatalf("bad repo filter: %+v", repo.lastFilter)
		}
	})

	t.Run("query defaults to relevance", func(t *testing.T) {
		_, err := svc.List(ctx, public.ListFilter{Query: "  oil  "})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if repo.lastFilter.Sort != "relevance" || repo.lastFilter.Query != "oil" {
			t.Fatalf("bad search filter: %+v", repo.lastFilter)
		}
	})

	t.Run("explicit sort wins over query default", func(t *testing.T) {
		_, err := svc.List(ctx, public.ListFilter{Query: "oil", Sort: "price_asc"})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if repo.lastFilter.Sort != "price_asc" {
			t.Fatalf("explicit sort lost: %+v", repo.lastFilter)
		}
	})

	t.Run("page converts to offset and beats offset", func(t *testing.T) {
		page, err := svc.List(ctx, public.ListFilter{Page: 3, Limit: 10, Offset: 99})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if repo.lastFilter.Offset != 20 || page.Meta.Offset != 20 || page.Meta.Page != 3 {
			t.Fatalf("page not applied: meta=%+v filter=%+v", page.Meta, repo.lastFilter)
		}
		if page.Meta.HasPrev != true {
			t.Fatalf("expected has_prev")
		}
	})

	t.Run("limit is capped", func(t *testing.T) {
		page, err := svc.List(ctx, public.ListFilter{Limit: 500})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if page.Meta.Limit != 100 || repo.lastFilter.Limit != 100 {
			t.Fatalf("limit not capped: %+v", page.Meta)
		}
	})

	t.Run("filters are trimmed and forwarded", func(t *testing.T) {
		after := time.Now().Add(-time.Hour)
		before := time.Now()
		_, err := svc.List(ctx, public.ListFilter{
			Query: "  dress  ", CategoryID: strptr("  "), CategorySlug: strptr(" dresses "),
			MinPriceCents: i64ptr(1000), MaxPriceCents: i64ptr(5000),
			InStock: boolptr(true), Size: " xl ", MinRating: f64ptr(4),
			CreatedAfter: &after, CreatedBefore: &before, Sort: "price_desc",
		})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		f := repo.lastFilter
		if f.Query != "dress" || f.CategoryID != nil || f.CategorySlug == nil || *f.CategorySlug != "dresses" {
			t.Fatalf("bad text filters: %+v", f)
		}
		if f.Size != "xl" || f.MinRating == nil || *f.MinRating != 4 || f.InStock == nil || !*f.InStock {
			t.Fatalf("bad typed filters: %+v", f)
		}
		if f.Sort != "price_desc" || f.MinPriceCents == nil || f.MaxPriceCents == nil {
			t.Fatalf("bad sort/price: %+v", f)
		}
	})

	t.Run("zero rating means no floor", func(t *testing.T) {
		_, err := svc.List(ctx, public.ListFilter{MinRating: f64ptr(0)})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if repo.lastFilter.MinRating != nil {
			t.Fatalf("expected nil rating, got %v", *repo.lastFilter.MinRating)
		}
	})
}

func TestListValidation(t *testing.T) {
	ctx := context.Background()
	svc := service.NewService(newFakeRepo(), fakeCategories{}, fakeVariants{})
	after := time.Now()
	before := after.Add(-time.Hour)

	cases := []struct {
		name string
		in   public.ListFilter
	}{
		{"negative limit", public.ListFilter{Limit: -1}},
		{"negative offset", public.ListFilter{Offset: -1}},
		{"negative page", public.ListFilter{Page: -1}},
		{"negative min price", public.ListFilter{MinPriceCents: i64ptr(-1)}},
		{"negative max price", public.ListFilter{MaxPriceCents: i64ptr(-1)}},
		{"inverted price range", public.ListFilter{MinPriceCents: i64ptr(5), MaxPriceCents: i64ptr(1)}},
		{"rating above five", public.ListFilter{MinRating: f64ptr(5.1)}},
		{"rating below zero", public.ListFilter{MinRating: f64ptr(-0.1)}},
		{"inverted dates", public.ListFilter{CreatedAfter: &after, CreatedBefore: &before}},
		{"unknown sort", public.ListFilter{Sort: "drop table products"}},
		{"relevance without query", public.ListFilter{Sort: "relevance"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.List(ctx, tc.in); apperr.CodeOf(err) != apperr.CodeValidation {
				t.Fatalf("expected VALIDATION_ERROR, got %v", err)
			}
		})
	}
}

func boolptr(v bool) *bool { return &v }

func TestDetailIncludesGalleryImages(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo(model.Product{
		ID: "prod-oil", Name: "Oil", Slug: "oil", PriceCents: 100,
		ImageURL: "https://cdn/primary.jpg", ImagePublicID: "primary",
	})
	repo.images["prod-oil"] = []model.Image{
		{ID: "img-1", ProductID: "prod-oil", URL: "https://cdn/primary.jpg", PublicID: "primary", Position: 0},
		{ID: "img-2", ProductID: "prod-oil", URL: "https://cdn/second.jpg", PublicID: "second", Position: 1},
		{ID: "img-3", ProductID: "prod-oil", Position: 2}, // still queued
	}
	svc := service.NewService(repo, fakeCategories{}, fakeVariants{})

	d, err := svc.GetByID(ctx, "prod-oil")
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if len(d.Images) != 2 {
		t.Fatalf("expected 2 ready images, got %+v", d.Images)
	}
	if !d.Images[0].IsPrimary {
		t.Fatalf("first image should be primary: %+v", d.Images[0])
	}
	if d.Images[1].IsPrimary {
		t.Fatalf("second image should not be primary: %+v", d.Images[1])
	}
	if d.Images[0].URL != "https://cdn/primary.jpg" || d.Images[1].Position != 1 {
		t.Fatalf("bad gallery: %+v", d.Images)
	}
}

func TestGalleryLifecycle(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo(model.Product{ID: "prod-oil", Name: "Oil", Slug: "oil", PriceCents: 100})
	svc := service.NewService(repo, fakeCategories{}, fakeVariants{})

	first, err := svc.AddImage(ctx, "prod-oil")
	if err != nil {
		t.Fatalf("add first: %v", err)
	}
	second, err := svc.AddImage(ctx, "prod-oil")
	if err != nil {
		t.Fatalf("add second: %v", err)
	}
	if first.Position != 0 || second.Position != 1 {
		t.Fatalf("positions not appended: %d %d", first.Position, second.Position)
	}
	if first.URL != "" {
		t.Fatalf("new row should be pending: %+v", first)
	}

	// Pending rows cannot be promoted.
	if _, err := svc.SetPrimaryImage(ctx, "prod-oil", first.ID); apperr.CodeOf(err) != apperr.CodeValidation {
		t.Fatalf("expected VALIDATION_ERROR, got %v", err)
	}

	// First completed upload fills the row and becomes primary because the
	// product had no image_url.
	if err := svc.CompleteImage(ctx, first.ID, "https://cdn/one.jpg", "one"); err != nil {
		t.Fatalf("complete first: %v", err)
	}
	if got := repo.byID["prod-oil"].ImageURL; got != "https://cdn/one.jpg" {
		t.Fatalf("first image not promoted: %q", got)
	}

	if err := svc.CompleteImage(ctx, second.ID, "https://cdn/two.jpg", "two"); err != nil {
		t.Fatalf("complete second: %v", err)
	}
	if got := repo.byID["prod-oil"].ImageURL; got != "https://cdn/one.jpg" {
		t.Fatalf("second completion must not steal the primary: %q", got)
	}

	// Admin can move the primary.
	p, err := svc.SetPrimaryImage(ctx, "prod-oil", second.ID)
	if err != nil {
		t.Fatalf("set primary: %v", err)
	}
	if p.ImageURL != "https://cdn/two.jpg" {
		t.Fatalf("primary not changed: %+v", p)
	}

	// Deleting the primary promotes the remaining ready image.
	if err := svc.DeleteImage(ctx, "prod-oil", second.ID); err != nil {
		t.Fatalf("delete primary: %v", err)
	}
	if got := repo.byID["prod-oil"].ImageURL; got != "https://cdn/one.jpg" {
		t.Fatalf("next image not promoted after delete: %q", got)
	}

	images, err := svc.ListImages(ctx, "prod-oil")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(images) != 1 || images[0].ID != first.ID || !images[0].IsPrimary {
		t.Fatalf("bad admin gallery: %+v", images)
	}
}

func TestGalleryValidationAndMissingRows(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo(model.Product{ID: "prod-oil", Name: "Oil", Slug: "oil", PriceCents: 100})
	svc := service.NewService(repo, fakeCategories{}, fakeVariants{})

	if _, err := svc.AddImage(ctx, "nope"); apperr.CodeOf(err) != apperr.CodeNotFound {
		t.Fatalf("expected NOT_FOUND for unknown product, got %v", err)
	}
	if _, err := svc.ListImages(ctx, " "); apperr.CodeOf(err) != apperr.CodeValidation {
		t.Fatalf("expected VALIDATION_ERROR for blank id, got %v", err)
	}
	if err := svc.DeleteImage(ctx, "prod-oil", "  "); apperr.CodeOf(err) != apperr.CodeValidation {
		t.Fatalf("expected VALIDATION_ERROR for blank image id, got %v", err)
	}
	if _, err := svc.SetPrimaryImage(ctx, "prod-oil", "missing"); apperr.CodeOf(err) != apperr.CodeNotFound {
		t.Fatalf("expected NOT_FOUND for unknown image, got %v", err)
	}

	// A job whose row was deleted while queued is a no-op, not an error.
	if err := svc.CompleteImage(ctx, "missing", "https://cdn/x.jpg", "x"); err != nil {
		t.Fatalf("missing row should complete silently: %v", err)
	}
}
