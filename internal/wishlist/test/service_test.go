package test

import (
	"context"
	"testing"

	productspublic "github.com/nikhea/malawi-e-commerce-store/internal/products/public"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/wishlist/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/wishlist/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/wishlist/service"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	bulkdto "github.com/nikhea/malawi-e-commerce-store/pkg/bulk/dto"
)

// fakeRepo is an in-memory link store.
type fakeRepo struct {
	links map[string]model.Item // user|product → item
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{links: map[string]model.Item{}}
}

func (f *fakeRepo) Add(_ context.Context, userID, productID string) (model.Item, bool, error) {
	k := userID + "|" + productID
	if i, ok := f.links[k]; ok {
		return i, false, nil // idempotent
	}
	i := model.Item{ID: "w-" + productID, UserID: userID, ProductID: productID}
	f.links[k] = i
	return i, true, nil
}

func (f *fakeRepo) AddMany(ctx context.Context, userID string, productIDs []string) ([]model.AddResult, int, error) {
	out := make([]model.AddResult, len(productIDs))
	for i, id := range productIDs {
		item, created, err := f.Add(ctx, userID, id)
		if err != nil {
			return nil, i, err
		}
		out[i] = model.AddResult{Item: item, Created: created}
	}
	return out, -1, nil
}

func (f *fakeRepo) Existing(_ context.Context, userID string, productIDs []string) (map[string]bool, error) {
	out := make(map[string]bool, len(productIDs))
	for _, id := range productIDs {
		if _, ok := f.links[userID+"|"+id]; ok {
			out[id] = true
		}
	}
	return out, nil
}

func (f *fakeRepo) RemoveMany(ctx context.Context, userID string, productIDs []string) (int, error) {
	for i, id := range productIDs {
		if err := f.Remove(ctx, userID, id); err != nil {
			return i, err
		}
	}
	return -1, nil
}

func (f *fakeRepo) ListByUser(_ context.Context, userID string, filter model.ListFilter) ([]model.Item, int64, error) {
	out := []model.Item{}
	for _, i := range f.links {
		if i.UserID == userID {
			out = append(out, i)
		}
	}
	total := int64(len(out))
	start := filter.Offset
	if start > len(out) {
		start = len(out)
	}
	end := len(out)
	if filter.Limit > 0 && start+filter.Limit < end {
		end = start + filter.Limit
	}
	return out[start:end], total, nil
}

func (f *fakeRepo) Remove(_ context.Context, userID, productID string) error {
	k := userID + "|" + productID
	if _, ok := f.links[k]; !ok {
		return apperr.NotFound("wishlist item not found")
	}
	delete(f.links, k)
	return nil
}

// fakeProducts knows "prod-1", "prod-2" and inactive "prod-off".
type fakeProducts struct{}

func (fakeProducts) detail(id string) (productspublic.Detail, error) {
	switch id {
	case "prod-1", "prod-2":
		return productspublic.Detail{
			Product: productspublic.Product{ID: id, Name: "Oil " + id, Slug: "oil-" + id, PriceCents: 1000, Currency: "USD", IsActive: true},
		}, nil
	case "prod-off":
		return productspublic.Detail{
			Product: productspublic.Product{ID: id, Name: "Old", Slug: "old", PriceCents: 100, Currency: "USD", IsActive: false},
		}, nil
	}
	return productspublic.Detail{}, apperr.NotFound("product not found")
}

func (f fakeProducts) Create(context.Context, productspublic.CreateProductInput) (productspublic.Product, error) {
	return productspublic.Product{}, nil
}
func (f fakeProducts) GetByID(_ context.Context, id string) (productspublic.Detail, error) {
	return f.detail(id)
}
func (f fakeProducts) GetBySlug(context.Context, string) (productspublic.Detail, error) {
	return productspublic.Detail{}, apperr.NotFound("product not found")
}
func (f fakeProducts) List(context.Context, productspublic.ListFilter) (productspublic.Page, error) {
	return productspublic.Page{}, nil
}
func (f fakeProducts) Update(context.Context, string, productspublic.UpdateProductInput) (productspublic.Product, error) {
	return productspublic.Product{}, nil
}
func (f fakeProducts) Delete(context.Context, string) error { return nil }
func (f fakeProducts) SetImage(context.Context, string, string, string) (productspublic.Product, error) {
	return productspublic.Product{}, nil
}
func (f fakeProducts) AddImage(context.Context, string) (productspublic.Image, error) {
	return productspublic.Image{}, nil
}
func (f fakeProducts) ListImages(context.Context, string) ([]productspublic.Image, error) {
	return nil, nil
}
func (f fakeProducts) DeleteImage(context.Context, string, string) error { return nil }
func (f fakeProducts) SetPrimaryImage(context.Context, string, string) (productspublic.Product, error) {
	return productspublic.Product{}, nil
}
func (f fakeProducts) CompleteImage(context.Context, string, string, string) error { return nil }

// fakeUsers knows "u1" only.
type fakeUsers struct{}

func (fakeUsers) Create(context.Context, userspublic.CreateUserInput) (userspublic.User, error) {
	return userspublic.User{}, nil
}
func (f fakeUsers) GetByID(_ context.Context, id string) (userspublic.User, error) {
	if id != "u1" {
		return userspublic.User{}, apperr.NotFound("user not found")
	}
	return userspublic.User{ID: "u1"}, nil
}
func (fakeUsers) GetByEmail(context.Context, string) (userspublic.User, error) {
	return userspublic.User{}, apperr.NotFound("user not found")
}
func (fakeUsers) GetCredentials(context.Context, string) (userspublic.Credentials, error) {
	return userspublic.Credentials{}, apperr.NotFound("user not found")
}
func (fakeUsers) SetRole(context.Context, string, userspublic.Role) (userspublic.User, error) {
	return userspublic.User{}, nil
}
func (fakeUsers) List(context.Context, userspublic.ListFilter) (userspublic.Page, error) {
	return userspublic.Page{}, nil
}

func (fakeUsers) SetEmailVerified(context.Context, string, bool) (userspublic.User, error) {
	return userspublic.User{}, nil
}
func (fakeUsers) SetPasswordHash(context.Context, string, string) (userspublic.User, error) {
	return userspublic.User{}, nil
}

func TestWishlist(t *testing.T) {
	ctx := context.Background()
	svc := service.NewService(newFakeRepo(), fakeProducts{}, fakeUsers{})

	item, err := svc.Add(ctx, "u1", "prod-1")
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if item.Product.Name != "Oil prod-1" || item.Product.PriceCents != 1000 {
		t.Fatalf("bad attach: %+v", item)
	}
	// Re-heart idempotent.
	if _, err := svc.Add(ctx, "u1", "prod-1"); err != nil {
		t.Fatalf("re-add: %v", err)
	}
	page, err := svc.ListMine(ctx, "u1", public.ListFilter{})
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("list: %v %+v", err, page)
	}
	if page.Meta.Limit != 12 || page.Meta.Total != 1 {
		t.Fatalf("bad meta: %+v", page.Meta)
	}
	if _, err := svc.ListMine(ctx, "u1", public.ListFilter{Sort: "random"}); apperr.CodeOf(err) != apperr.CodeValidation {
		t.Fatalf("bad sort: expected VALIDATION_ERROR, got %v", err)
	}
	// Unknown product rejected.
	if _, err := svc.Add(ctx, "u1", "ghost"); apperr.CodeOf(err) != apperr.CodeNotFound {
		t.Fatalf("expected NOT_FOUND, got %v", err)
	}
	if err := svc.Remove(ctx, "u1", "prod-1"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if err := svc.Remove(ctx, "u1", "prod-1"); apperr.CodeOf(err) != apperr.CodeNotFound {
		t.Fatalf("expected NOT_FOUND, got %v", err)
	}
}

func TestBulkAdd(t *testing.T) {
	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		repo := newFakeRepo()
		svc := service.NewService(repo, fakeProducts{}, fakeUsers{})
		results, err := svc.BulkAdd(ctx, "u1", []string{"prod-1", "prod-2"}, false)
		if err != nil {
			t.Fatalf("bulk add: %v", err)
		}
		if len(results) != 2 || results[0].Status != "ok" || results[1].Status != "ok" {
			t.Fatalf("bad results: %+v", results)
		}
		if results[0].Data == nil || results[0].Data.Product.Name != "Oil prod-1" {
			t.Fatalf("product context missing: %+v", results[0].Data)
		}
		if len(repo.links) != 2 {
			t.Fatalf("expected 2 links, got %d", len(repo.links))
		}
	})

	t.Run("already hearted", func(t *testing.T) {
		repo := newFakeRepo()
		svc := service.NewService(repo, fakeProducts{}, fakeUsers{})
		if _, err := svc.Add(ctx, "u1", "prod-1"); err != nil {
			t.Fatalf("seed: %v", err)
		}
		results, err := svc.BulkAdd(ctx, "u1", []string{"prod-1"}, false)
		if err != nil {
			t.Fatalf("bulk add: %v", err)
		}
		if results[0].Status != "ok" || !results[0].AlreadyExists {
			t.Fatalf("expected alreadyExists, got %+v", results[0])
		}
	})

	t.Run("unknown and inactive are per item", func(t *testing.T) {
		svc := service.NewService(newFakeRepo(), fakeProducts{}, fakeUsers{})
		results, err := svc.BulkAdd(ctx, "u1", []string{"prod-1", "ghost", "prod-off"}, false)
		if err != nil {
			t.Fatalf("bulk add: %v", err)
		}
		if results[0].Status != "ok" {
			t.Fatalf("first should succeed: %+v", results[0])
		}
		for _, i := range []int{1, 2} {
			if results[i].Status != "error" || results[i].Error.Code != apperr.CodeNotFound {
				t.Fatalf("item %d expected NOT_FOUND, got %+v", i, results[i])
			}
		}
	})

	t.Run("duplicates reject the batch", func(t *testing.T) {
		svc := service.NewService(newFakeRepo(), fakeProducts{}, fakeUsers{})
		if _, err := svc.BulkAdd(ctx, "u1", []string{"prod-1", "prod-1"}, false); apperr.CodeOf(err) != apperr.CodeDuplicateItem {
			t.Fatalf("expected DUPLICATE_ITEM, got %v", err)
		}
	})

	t.Run("cap enforced", func(t *testing.T) {
		svc := service.NewService(newFakeRepo(), fakeProducts{}, fakeUsers{})
		ids := make([]string, 51)
		for i := range ids {
			ids[i] = "prod-1"
		}
		// Duplicates are checked after the cap, so this is the cap error.
		if _, err := svc.BulkAdd(ctx, "u1", ids, false); apperr.CodeOf(err) != apperr.CodeBulkLimitExceeded {
			t.Fatalf("expected BULK_LIMIT_EXCEEDED, got %v", err)
		}
	})

	t.Run("atomic rolls back", func(t *testing.T) {
		repo := newFakeRepo()
		svc := service.NewService(repo, fakeProducts{}, fakeUsers{})
		results, err := svc.BulkAdd(ctx, "u1", []string{"prod-1", "ghost"}, true)
		if err != nil {
			t.Fatalf("bulk add: %v", err)
		}
		for _, r := range results {
			if r.Status != "rolled_back" {
				t.Fatalf("expected rolled_back, got %+v", r)
			}
		}
		if results[1].Error == nil || results[1].Error.Code != apperr.CodeNotFound {
			t.Fatalf("failing item should carry the error: %+v", results[1])
		}
		if len(repo.links) != 0 {
			t.Fatalf("atomic rollback wrote links: %d", len(repo.links))
		}
	})

	t.Run("atomic success", func(t *testing.T) {
		repo := newFakeRepo()
		svc := service.NewService(repo, fakeProducts{}, fakeUsers{})
		results, err := svc.BulkAdd(ctx, "u1", []string{"prod-1", "prod-2"}, true)
		if err != nil {
			t.Fatalf("bulk add: %v", err)
		}
		for _, r := range results {
			if r.Status != "ok" {
				t.Fatalf("expected ok, got %+v", r)
			}
		}
		if len(repo.links) != 2 {
			t.Fatalf("expected 2 links, got %d", len(repo.links))
		}
	})
}

func TestBulkRemove(t *testing.T) {
	ctx := context.Background()

	seed := func(t *testing.T, productIDs ...string) (*fakeRepo, public.Service) {
		t.Helper()
		repo := newFakeRepo()
		svc := service.NewService(repo, fakeProducts{}, fakeUsers{})
		for _, id := range productIDs {
			if _, err := svc.Add(ctx, "u1", id); err != nil {
				t.Fatalf("seed %s: %v", id, err)
			}
		}
		return repo, svc
	}

	t.Run("happy path", func(t *testing.T) {
		repo, svc := seed(t, "prod-1", "prod-2")
		results, err := svc.BulkRemove(ctx, "u1", []string{"prod-1", "prod-2"}, false, false)
		if err != nil {
			t.Fatalf("bulk remove: %v", err)
		}
		for _, r := range results {
			if r.Status != "ok" || r.AlreadyRemoved {
				t.Fatalf("expected ok, got %+v", r)
			}
		}
		if len(repo.links) != 0 {
			t.Fatalf("links left behind: %d", len(repo.links))
		}
	})

	t.Run("unknown and empty ids are per item", func(t *testing.T) {
		_, svc := seed(t, "prod-1")
		results, err := svc.BulkRemove(ctx, "u1", []string{"ghost", ""}, false, false)
		if err != nil {
			t.Fatalf("bulk remove: %v", err)
		}
		if results[0].Error == nil || results[0].Error.Code != apperr.CodeNotFound {
			t.Fatalf("unknown product expected NOT_FOUND: %+v", results[0])
		}
		if results[1].Error == nil || results[1].Error.Code != apperr.CodeValidation {
			t.Fatalf("empty id expected VALIDATION_ERROR: %+v", results[1])
		}
	})

	t.Run("already removed is idempotent", func(t *testing.T) {
		_, svc := seed(t, "prod-1")
		if err := svc.Remove(ctx, "u1", "prod-1"); err != nil {
			t.Fatalf("seed remove: %v", err)
		}
		results, err := svc.BulkRemove(ctx, "u1", []string{"prod-1"}, false, false)
		if err != nil {
			t.Fatalf("bulk remove: %v", err)
		}
		if results[0].Status != "ok" || !results[0].AlreadyRemoved {
			t.Fatalf("expected alreadyRemoved, got %+v", results[0])
		}
	})

	t.Run("dry run writes nothing", func(t *testing.T) {
		repo, svc := seed(t, "prod-1")
		results, err := svc.BulkRemove(ctx, "u1", []string{"prod-1", "prod-2"}, false, true)
		if err != nil {
			t.Fatalf("dry run: %v", err)
		}
		if !results[0].WouldDelete || results[0].AlreadyRemoved {
			t.Fatalf("expected wouldDelete for the hearted product: %+v", results[0])
		}
		if !results[1].AlreadyRemoved || results[1].WouldDelete {
			t.Fatalf("expected alreadyRemoved for the unhearted product: %+v", results[1])
		}
		if len(repo.links) != 1 {
			t.Fatalf("dry run wrote: %d links", len(repo.links))
		}
	})

	t.Run("duplicates reject the batch", func(t *testing.T) {
		_, svc := seed(t)
		if _, err := svc.BulkRemove(ctx, "u1", []string{"prod-1", "prod-1"}, false, false); apperr.CodeOf(err) != apperr.CodeDuplicateItem {
			t.Fatalf("expected DUPLICATE_ITEM, got %v", err)
		}
	})

	t.Run("cap enforced", func(t *testing.T) {
		_, svc := seed(t)
		ids := make([]string, 51)
		for i := range ids {
			ids[i] = "prod-1"
		}
		if _, err := svc.BulkRemove(ctx, "u1", ids, false, false); apperr.CodeOf(err) != apperr.CodeBulkLimitExceeded {
			t.Fatalf("expected BULK_LIMIT_EXCEEDED, got %v", err)
		}
	})

	t.Run("atomic rolls back", func(t *testing.T) {
		repo, svc := seed(t, "prod-1")
		results, err := svc.BulkRemove(ctx, "u1", []string{"prod-1", "ghost"}, true, false)
		if err != nil {
			t.Fatalf("bulk remove: %v", err)
		}
		for _, r := range results {
			if r.Status != "rolled_back" {
				t.Fatalf("expected rolled_back, got %+v", r)
			}
		}
		if results[1].Error == nil || results[1].Error.Code != apperr.CodeNotFound {
			t.Fatalf("failing item should carry the error: %+v", results[1])
		}
		if len(repo.links) != 1 {
			t.Fatalf("atomic rollback removed the link: %d", len(repo.links))
		}
	})
}

// Bulk admin writes are not exercised by this module's tests.
func (fakeProducts) BulkCreate(context.Context, []productspublic.CreateProductInput, bool, bool) ([]bulkdto.BulkResult[productspublic.Product], error) {
	return nil, nil
}
func (fakeProducts) BulkUpdate(context.Context, []productspublic.BulkUpdateItem, bool, bool) ([]bulkdto.BulkResult[productspublic.Product], error) {
	return nil, nil
}
func (fakeProducts) BulkDelete(context.Context, []string, bool, bool) ([]bulkdto.BulkResult[bulkdto.Empty], error) {
	return nil, nil
}
