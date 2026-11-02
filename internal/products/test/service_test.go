package test

import (
	"context"
	"errors"
	"fmt"
	"github.com/nikhea/malawi-e-commerce-store/pkg/bulk"
	"testing"
	"time"

	categoriespublic "github.com/nikhea/malawi-e-commerce-store/internal/categories/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/products/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/products/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/products/service"
	variantspublic "github.com/nikhea/malawi-e-commerce-store/internal/variants/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
)

var errBulk = errors.New("bulk write failed")

// fakeRepo is an in-memory product store. No real DB.
type fakeRepo struct {
	byID       map[string]model.Product
	bySlug     map[string]string
	lastFilter model.ListFilter
	images     map[string][]model.Image
	nextImage  int
	// Blocker counters are opt-in: tests seed only the rows they need.
	orderItems map[string]int
	variants   map[string]int
	cartLines  map[string]int
	wishlists  map[string]int
	failBulkAt int // atomic batch index that fails; -1 disables
}

func newFakeRepo(seed ...model.Product) *fakeRepo {
	f := &fakeRepo{
		byID:       map[string]model.Product{},
		bySlug:     map[string]string{},
		images:     map[string][]model.Image{},
		orderItems: map[string]int{},
		variants:   map[string]int{},
		cartLines:  map[string]int{},
		wishlists:  map[string]int{},
		failBulkAt: -1,
	}
	for _, p := range seed {
		f.byID[p.ID] = p
		f.bySlug[p.Slug] = p.ID
	}
	return f
}

func (f *fakeRepo) snapshot() (map[string]model.Product, map[string]string) {
	byID := make(map[string]model.Product, len(f.byID))
	for k, v := range f.byID {
		byID[k] = v
	}
	bySlug := make(map[string]string, len(f.bySlug))
	for k, v := range f.bySlug {
		bySlug[k] = v
	}
	return byID, bySlug
}

func (f *fakeRepo) restore(byID map[string]model.Product, bySlug map[string]string) {
	f.byID, f.bySlug = byID, bySlug
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
	wanted := map[string]bool{}
	for _, id := range filter.IDs {
		wanted[id] = true
	}
	for _, p := range f.byID {
		if len(wanted) > 0 && !wanted[p.ID] {
			continue
		}
		out = append(out, p)
	}
	return out, int64(len(out)), nil
}

func (f *fakeRepo) CreateMany(_ context.Context, items []model.Product) ([]model.Product, int, error) {
	byID, bySlug := f.snapshot()
	out := make([]model.Product, 0, len(items))
	for i, p := range items {
		if i == f.failBulkAt {
			f.restore(byID, bySlug)
			return nil, i, apperr.Internal(errBulk)
		}
		created, err := f.Create(context.Background(), p)
		if err != nil {
			f.restore(byID, bySlug)
			return nil, i, err
		}
		out = append(out, created)
	}
	return out, -1, nil
}

func (f *fakeRepo) UpdateMany(_ context.Context, items []model.Product) ([]model.Product, int, error) {
	byID, bySlug := f.snapshot()
	out := make([]model.Product, 0, len(items))
	for i, p := range items {
		if i == f.failBulkAt {
			f.restore(byID, bySlug)
			return nil, i, apperr.Internal(errBulk)
		}
		updated, err := f.Update(context.Background(), p)
		if err != nil {
			f.restore(byID, bySlug)
			return nil, i, err
		}
		out = append(out, updated)
	}
	return out, -1, nil
}

func (f *fakeRepo) DeleteMany(_ context.Context, ids []string) (int, error) {
	byID, bySlug := f.snapshot()
	for i, id := range ids {
		if i == f.failBulkAt {
			f.restore(byID, bySlug)
			return i, apperr.Internal(errBulk)
		}
		if err := f.Delete(context.Background(), id); err != nil {
			f.restore(byID, bySlug)
			return i, err
		}
	}
	return -1, nil
}

func (f *fakeRepo) Blockers(_ context.Context, ids []string) (map[string]model.Blockers, error) {
	out := make(map[string]model.Blockers, len(ids))
	for _, id := range ids {
		if _, ok := f.byID[id]; !ok {
			continue
		}
		out[id] = model.Blockers{
			OrderItems:    f.orderItems[id],
			Variants:      f.variants[id],
			CartLines:     f.cartLines[id],
			WishlistItems: f.wishlists[id],
		}
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

func (fakeVariants) BulkCreate(context.Context, string, []variantspublic.CreateVariantInput, bool, bool) ([]bulk.BulkResult[variantspublic.Variant], error) {
	return nil, nil
}

func (fakeVariants) BulkUpdate(context.Context, []variantspublic.BulkUpdateItem, bool, bool) ([]bulk.BulkResult[variantspublic.Variant], error) {
	return nil, nil
}

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

func TestBulkCreate(t *testing.T) {
	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		repo := newFakeRepo()
		svc := service.NewService(repo, fakeCategories{}, fakeVariants{})

		results, err := svc.BulkCreate(ctx, []public.CreateProductInput{
			{Name: "Sunseed Oil", PriceCents: 100},
			{Name: "Soap", PriceCents: 200, CategoryID: strptr("cat-1")},
		}, false, false)
		if err != nil {
			t.Fatalf("bulk create: %v", err)
		}
		if len(results) != 2 || results[0].Status != "ok" || results[1].Status != "ok" {
			t.Fatalf("bad results: %+v", results)
		}
		if results[0].ID != "sunseed-oil" || results[0].Data == nil || results[0].Data.Slug != "sunseed-oil" {
			t.Fatalf("bad first row: %+v", results[0])
		}
		if len(repo.byID) != 2 {
			t.Fatalf("expected two writes, got %d", len(repo.byID))
		}
	})

	t.Run("duplicate slug rejects whole request", func(t *testing.T) {
		repo := newFakeRepo()
		svc := service.NewService(repo, fakeCategories{}, fakeVariants{})
		_, err := svc.BulkCreate(ctx, []public.CreateProductInput{
			{Name: "Oil", PriceCents: 100},
			{Name: "Oil", PriceCents: 100},
		}, false, false)
		if apperr.CodeOf(err) != apperr.CodeDuplicateItem {
			t.Fatalf("expected DUPLICATE_ITEM, got %v", err)
		}
		if len(repo.byID) != 0 {
			t.Fatalf("duplicate request wrote rows: %+v", repo.byID)
		}
	})

	t.Run("cap", func(t *testing.T) {
		items := make([]public.CreateProductInput, 51)
		for i := range items {
			items[i] = public.CreateProductInput{Name: fmt.Sprintf("P%d", i), PriceCents: 1}
		}
		svc := service.NewService(newFakeRepo(), fakeCategories{}, fakeVariants{})
		if _, err := svc.BulkCreate(ctx, items, false, false); apperr.CodeOf(err) != apperr.CodeBulkLimitExceeded {
			t.Fatalf("expected BULK_LIMIT_EXCEEDED, got %v", err)
		}
	})

	t.Run("slug conflict is per item", func(t *testing.T) {
		repo := newFakeRepo(model.Product{ID: "existing", Name: "Oil", Slug: "oil"})
		svc := service.NewService(repo, fakeCategories{}, fakeVariants{})
		results, err := svc.BulkCreate(ctx, []public.CreateProductInput{
			{Name: "Oil", PriceCents: 100},
			{Name: "Soap", PriceCents: 100},
		}, false, false)
		if err != nil {
			t.Fatalf("bulk create: %v", err)
		}
		if results[0].Status != "error" || results[0].Error == nil || results[0].Error.Code != apperr.CodeConflict {
			t.Fatalf("expected per-item CONFLICT: %+v", results[0])
		}
		if results[1].Status != "ok" {
			t.Fatalf("second item should succeed: %+v", results[1])
		}
	})

	t.Run("dry run writes nothing", func(t *testing.T) {
		repo := newFakeRepo()
		svc := service.NewService(repo, fakeCategories{}, fakeVariants{})
		results, err := svc.BulkCreate(ctx, []public.CreateProductInput{{Name: "Oil", PriceCents: 100}}, false, true)
		if err != nil {
			t.Fatalf("dry run: %v", err)
		}
		if results[0].Data == nil || results[0].Data.ID != "" || results[0].Data.Slug != "oil" {
			t.Fatalf("dry run should project without an id: %+v", results[0].Data)
		}
		if len(repo.byID) != 0 {
			t.Fatalf("dry run wrote rows: %+v", repo.byID)
		}
	})

	t.Run("atomic rolls back", func(t *testing.T) {
		repo := newFakeRepo()
		repo.failBulkAt = 1
		svc := service.NewService(repo, fakeCategories{}, fakeVariants{})
		results, err := svc.BulkCreate(ctx, []public.CreateProductInput{
			{Name: "Oil", PriceCents: 100},
			{Name: "Soap", PriceCents: 100},
		}, true, false)
		if err != nil {
			t.Fatalf("atomic create: %v", err)
		}
		for _, r := range results {
			if r.Status != "rolled_back" {
				t.Fatalf("expected rollback: %+v", results)
			}
		}
		if len(repo.byID) != 0 {
			t.Fatalf("atomic failure leaked rows: %+v", repo.byID)
		}
	})
}

func TestBulkUpdate(t *testing.T) {
	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		repo := newFakeRepo(
			model.Product{ID: "p1", Name: "One", Slug: "one", PriceCents: 100},
			model.Product{ID: "p2", Name: "Two", Slug: "two", PriceCents: 200},
		)
		svc := service.NewService(repo, fakeCategories{}, fakeVariants{})
		results, err := svc.BulkUpdate(ctx, []public.BulkUpdateItem{
			{ID: "p1", Patch: public.UpdateProductInput{Name: strptr("One v2")}},
			{ID: "p2", Patch: public.UpdateProductInput{PriceCents: i64ptr(250)}},
		}, false, false)
		if err != nil {
			t.Fatalf("bulk update: %v", err)
		}
		if results[0].Data.Name != "One v2" || results[1].Data.PriceCents != 250 {
			t.Fatalf("bad results: %+v", results)
		}
		if repo.byID["p1"].Name != "One v2" || repo.byID["p2"].PriceCents != 250 {
			t.Fatalf("writes missing: %+v", repo.byID)
		}
	})

	t.Run("per-item validation and not found", func(t *testing.T) {
		repo := newFakeRepo(model.Product{ID: "p1", Name: "One", Slug: "one"})
		svc := service.NewService(repo, fakeCategories{}, fakeVariants{})
		results, err := svc.BulkUpdate(ctx, []public.BulkUpdateItem{
			{ID: "p1", Patch: public.UpdateProductInput{}},
			{ID: "ghost", Patch: public.UpdateProductInput{Name: strptr("X")}},
		}, false, false)
		if err != nil {
			t.Fatalf("bulk update: %v", err)
		}
		if results[0].Error == nil || results[0].Error.Code != apperr.CodeValidation {
			t.Fatalf("empty patch should be VALIDATION_ERROR: %+v", results[0])
		}
		if results[1].Error == nil || results[1].Error.Code != apperr.CodeNotFound {
			t.Fatalf("unknown id should be NOT_FOUND: %+v", results[1])
		}
	})

	t.Run("duplicate ids reject whole request", func(t *testing.T) {
		svc := service.NewService(newFakeRepo(), fakeCategories{}, fakeVariants{})
		_, err := svc.BulkUpdate(ctx, []public.BulkUpdateItem{
			{ID: "p1", Patch: public.UpdateProductInput{Name: strptr("A")}},
			{ID: "p1", Patch: public.UpdateProductInput{Name: strptr("B")}},
		}, false, false)
		if apperr.CodeOf(err) != apperr.CodeDuplicateItem {
			t.Fatalf("expected DUPLICATE_ITEM, got %v", err)
		}
	})

	t.Run("dry run writes nothing", func(t *testing.T) {
		repo := newFakeRepo(model.Product{ID: "p1", Name: "One", Slug: "one"})
		svc := service.NewService(repo, fakeCategories{}, fakeVariants{})
		results, err := svc.BulkUpdate(ctx, []public.BulkUpdateItem{
			{ID: "p1", Patch: public.UpdateProductInput{Name: strptr("Projected")}},
		}, false, true)
		if err != nil {
			t.Fatalf("dry run: %v", err)
		}
		if results[0].Data.Name != "Projected" || repo.byID["p1"].Name != "One" {
			t.Fatalf("dry run wrote: result=%+v row=%+v", results[0], repo.byID["p1"])
		}
	})

	t.Run("atomic rolls back", func(t *testing.T) {
		repo := newFakeRepo(
			model.Product{ID: "p1", Name: "One", Slug: "one"},
			model.Product{ID: "p2", Name: "Two", Slug: "two"},
		)
		repo.failBulkAt = 1
		svc := service.NewService(repo, fakeCategories{}, fakeVariants{})
		results, err := svc.BulkUpdate(ctx, []public.BulkUpdateItem{
			{ID: "p1", Patch: public.UpdateProductInput{Name: strptr("A")}},
			{ID: "p2", Patch: public.UpdateProductInput{Name: strptr("B")}},
		}, true, false)
		if err != nil {
			t.Fatalf("atomic update: %v", err)
		}
		for _, r := range results {
			if r.Status != "rolled_back" {
				t.Fatalf("expected rollback: %+v", results)
			}
		}
		if repo.byID["p1"].Name != "One" {
			t.Fatalf("rollback lost original: %+v", repo.byID["p1"])
		}
	})
}

func TestBulkDelete(t *testing.T) {
	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		repo := newFakeRepo(
			model.Product{ID: "p1", Name: "One", Slug: "one"},
			model.Product{ID: "p2", Name: "Two", Slug: "two"},
		)
		svc := service.NewService(repo, fakeCategories{}, fakeVariants{})
		results, err := svc.BulkDelete(ctx, []string{"p1", "p2"}, false, false)
		if err != nil {
			t.Fatalf("bulk delete: %v", err)
		}
		if results[0].Status != "ok" || results[1].Status != "ok" || len(repo.byID) != 0 {
			t.Fatalf("bad delete: %+v rows=%+v", results, repo.byID)
		}
	})

	t.Run("order history blocks per item", func(t *testing.T) {
		repo := newFakeRepo(model.Product{ID: "p1", Name: "One", Slug: "one"})
		repo.orderItems["p1"] = 2
		svc := service.NewService(repo, fakeCategories{}, fakeVariants{})
		results, err := svc.BulkDelete(ctx, []string{"p1"}, false, false)
		if err != nil {
			t.Fatalf("bulk delete: %v", err)
		}
		if results[0].Status != "error" || results[0].Error == nil || results[0].Error.Code != apperr.CodeConflict {
			t.Fatalf("expected CONFLICT: %+v", results[0])
		}
		if _, ok := repo.byID["p1"]; !ok {
			t.Fatal("blocked product was deleted")
		}
	})

	t.Run("dry run reports blockers", func(t *testing.T) {
		repo := newFakeRepo(
			model.Product{ID: "p1", Name: "One", Slug: "one"},
			model.Product{ID: "p2", Name: "Two", Slug: "two"},
		)
		repo.orderItems["p1"] = 2
		repo.variants["p1"] = 1
		repo.cartLines["p1"] = 3
		repo.wishlists["p1"] = 4
		svc := service.NewService(repo, fakeCategories{}, fakeVariants{})
		results, err := svc.BulkDelete(ctx, []string{"p1", "p2"}, false, true)
		if err != nil {
			t.Fatalf("dry run: %v", err)
		}
		if results[0].WouldDelete || len(results[0].Blockers) != 4 {
			t.Fatalf("bad blockers: %+v", results[0])
		}
		if !results[1].WouldDelete || len(results[1].Blockers) != 0 {
			t.Fatalf("clean product should be deletable: %+v", results[1])
		}
		if len(repo.byID) != 2 {
			t.Fatal("dry run deleted rows")
		}
	})

	t.Run("cap is 25", func(t *testing.T) {
		ids := make([]string, 26)
		for i := range ids {
			ids[i] = fmt.Sprintf("p%d", i)
		}
		svc := service.NewService(newFakeRepo(), fakeCategories{}, fakeVariants{})
		if _, err := svc.BulkDelete(ctx, ids, false, false); apperr.CodeOf(err) != apperr.CodeBulkLimitExceeded {
			t.Fatalf("expected BULK_LIMIT_EXCEEDED, got %v", err)
		}
	})

	t.Run("missing and duplicate ids", func(t *testing.T) {
		repo := newFakeRepo(model.Product{ID: "p1", Name: "One", Slug: "one"})
		svc := service.NewService(repo, fakeCategories{}, fakeVariants{})
		results, err := svc.BulkDelete(ctx, []string{"ghost"}, false, false)
		if err != nil {
			t.Fatalf("bulk delete: %v", err)
		}
		if results[0].Error == nil || results[0].Error.Code != apperr.CodeNotFound {
			t.Fatalf("expected NOT_FOUND: %+v", results[0])
		}
		if _, err := svc.BulkDelete(ctx, []string{"p1", "p1"}, false, false); apperr.CodeOf(err) != apperr.CodeDuplicateItem {
			t.Fatalf("expected DUPLICATE_ITEM, got %v", err)
		}
	})

	t.Run("atomic blocked batch rolls back", func(t *testing.T) {
		repo := newFakeRepo(
			model.Product{ID: "p1", Name: "One", Slug: "one"},
			model.Product{ID: "p2", Name: "Two", Slug: "two"},
		)
		repo.orderItems["p2"] = 1
		svc := service.NewService(repo, fakeCategories{}, fakeVariants{})
		results, err := svc.BulkDelete(ctx, []string{"p1", "p2"}, true, false)
		if err != nil {
			t.Fatalf("atomic delete: %v", err)
		}
		for _, r := range results {
			if r.Status != "rolled_back" {
				t.Fatalf("expected rollback: %+v", results)
			}
		}
		if len(repo.byID) != 2 {
			t.Fatalf("atomic blocked delete removed rows: %+v", repo.byID)
		}
	})
}

func TestListByIDs(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo(
		model.Product{ID: "p1", Name: "One", Slug: "one"},
		model.Product{ID: "p2", Name: "Two", Slug: "two"},
		model.Product{ID: "p3", Name: "Three", Slug: "three"},
	)
	svc := service.NewService(repo, fakeCategories{}, fakeVariants{})

	page, err := svc.List(ctx, public.ListFilter{IDs: []string{" p1 ", "p3"}, ActiveOnly: true})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if page.Meta.Total != 2 {
		t.Fatalf("expected 2 rows, got %d", page.Meta.Total)
	}
	if len(repo.lastFilter.IDs) != 2 || repo.lastFilter.IDs[0] != "p1" || repo.lastFilter.IDs[1] != "p3" {
		t.Fatalf("ids not forwarded: %+v", repo.lastFilter.IDs)
	}
}
