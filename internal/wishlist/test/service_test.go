package test

import (
	"context"
	"testing"

	productspublic "github.com/nikhea/malawi-e-commerce-store/internal/products/public"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/wishlist/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/wishlist/service"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
)

// fakeRepo is an in-memory link store.
type fakeRepo struct {
	links map[string]model.Item // user|product → item
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{links: map[string]model.Item{}}
}

func (f *fakeRepo) Add(_ context.Context, userID, productID string) (model.Item, error) {
	k := userID + "|" + productID
	if i, ok := f.links[k]; ok {
		return i, nil // idempotent
	}
	i := model.Item{ID: "w-" + productID, UserID: userID, ProductID: productID}
	f.links[k] = i
	return i, nil
}

func (f *fakeRepo) ListByUser(_ context.Context, userID string) ([]model.Item, error) {
	out := []model.Item{}
	for _, i := range f.links {
		if i.UserID == userID {
			out = append(out, i)
		}
	}
	return out, nil
}

func (f *fakeRepo) Remove(_ context.Context, userID, productID string) error {
	k := userID + "|" + productID
	if _, ok := f.links[k]; !ok {
		return apperr.NotFound("wishlist item not found")
	}
	delete(f.links, k)
	return nil
}

// fakeProducts knows "prod-1" only.
type fakeProducts struct{}

func (fakeProducts) detail(id string) (productspublic.Detail, error) {
	if id != "prod-1" {
		return productspublic.Detail{}, apperr.NotFound("product not found")
	}
	return productspublic.Detail{
		Product: productspublic.Product{ID: "prod-1", Name: "Oil", Slug: "oil", PriceCents: 1000, Currency: "MWK"},
	}, nil
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
func (f fakeProducts) List(context.Context, productspublic.ListFilter) ([]productspublic.Product, error) {
	return nil, nil
}
func (f fakeProducts) Update(context.Context, string, productspublic.UpdateProductInput) (productspublic.Product, error) {
	return productspublic.Product{}, nil
}
func (f fakeProducts) Delete(context.Context, string) error { return nil }
func (f fakeProducts) SetImage(context.Context, string, string, string) (productspublic.Product, error) {
	return productspublic.Product{}, nil
}

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
	if item.Product.Name != "Oil" || item.Product.PriceCents != 1000 {
		t.Fatalf("bad attach: %+v", item)
	}
	// Re-heart idempotent.
	if _, err := svc.Add(ctx, "u1", "prod-1"); err != nil {
		t.Fatalf("re-add: %v", err)
	}
	items, err := svc.ListMine(ctx, "u1")
	if err != nil || len(items) != 1 {
		t.Fatalf("list: %v %+v", err, items)
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
