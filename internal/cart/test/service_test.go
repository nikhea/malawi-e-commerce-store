package test

import (
	"context"
	"testing"

	"github.com/nikhea/malawi-e-commerce-store/internal/cart/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/cart/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/cart/service"
	productspublic "github.com/nikhea/malawi-e-commerce-store/internal/products/public"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
)

// fakeRepo is an in-memory cart store.
type fakeRepo struct {
	carts map[string]model.Cart   // userID → cart
	items map[string][]model.Item // cartID → items
	seq   int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{carts: map[string]model.Cart{}, items: map[string][]model.Item{}}
}

func (f *fakeRepo) nextID() string {
	f.seq++
	return string(rune('a'+f.seq)) + "-id"
}

func (f *fakeRepo) GetOrCreateCart(_ context.Context, userID string) (model.Cart, error) {
	if c, ok := f.carts[userID]; ok {
		return c, nil
	}
	c := model.Cart{ID: "cart-" + userID, UserID: userID}
	f.carts[userID] = c
	return c, nil
}

func (f *fakeRepo) ListItems(_ context.Context, cartID string) ([]model.Item, error) {
	return f.items[cartID], nil
}

func (f *fakeRepo) UpsertItem(_ context.Context, i model.Item) (model.Item, error) {
	for n, e := range f.items[i.CartID] {
		if e.ProductID == i.ProductID && ptrStr(e.VariantID) == ptrStr(i.VariantID) {
			e.Qty += i.Qty
			f.items[i.CartID][n] = e
			return e, nil
		}
	}
	i.ID = f.nextID()
	f.items[i.CartID] = append(f.items[i.CartID], i)
	return i, nil
}

func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (f *fakeRepo) GetItem(_ context.Context, cartID, itemID string) (model.Item, error) {
	for _, i := range f.items[cartID] {
		if i.ID == itemID {
			return i, nil
		}
	}
	return model.Item{}, apperr.NotFound("cart item not found")
}

func (f *fakeRepo) SetQty(_ context.Context, cartID, itemID string, qty int) (model.Item, error) {
	for n, i := range f.items[cartID] {
		if i.ID == itemID {
			i.Qty = qty
			f.items[cartID][n] = i
			return i, nil
		}
	}
	return model.Item{}, apperr.NotFound("cart item not found")
}

func (f *fakeRepo) DeleteItem(_ context.Context, cartID, itemID string) error {
	for n, i := range f.items[cartID] {
		if i.ID == itemID {
			f.items[cartID] = append(f.items[cartID][:n], f.items[cartID][n+1:]...)
			return nil
		}
	}
	return apperr.NotFound("cart item not found")
}

func (f *fakeRepo) Clear(_ context.Context, cartID string) error {
	f.items[cartID] = nil
	return nil
}

func (f *fakeRepo) Touch(_ context.Context, _ string) error { return nil }

// fakeProducts serves one active product with one variant, one inactive.
type fakeProducts struct{}

func detailFor(id string) (productspublic.Detail, error) {
	switch id {
	case "prod-1":
		return productspublic.Detail{
			Product:  productspublic.Product{ID: "prod-1", Name: "Oil", PriceCents: 1000, Currency: "MWK", IsActive: true},
			Variants: []productspublic.VariantView{{ID: "var-1", Name: "500ml", SKU: "OIL-500", PriceCents: 2500, IsActive: true}},
		}, nil
	case "prod-off":
		return productspublic.Detail{
			Product: productspublic.Product{ID: "prod-off", Name: "Old", PriceCents: 100, Currency: "MWK", IsActive: false},
		}, nil
	}
	return productspublic.Detail{}, apperr.NotFound("product not found")
}

func (fakeProducts) Create(context.Context, productspublic.CreateProductInput) (productspublic.Product, error) {
	return productspublic.Product{}, nil
}
func (fakeProducts) GetByID(_ context.Context, id string) (productspublic.Detail, error) {
	return detailFor(id)
}
func (fakeProducts) GetBySlug(context.Context, string) (productspublic.Detail, error) {
	return productspublic.Detail{}, apperr.NotFound("product not found")
}
func (fakeProducts) List(context.Context, productspublic.ListFilter) ([]productspublic.Product, error) {
	return nil, nil
}
func (fakeProducts) Update(context.Context, string, productspublic.UpdateProductInput) (productspublic.Product, error) {
	return productspublic.Product{}, nil
}
func (fakeProducts) Delete(context.Context, string) error { return nil }
func (fakeProducts) SetImage(context.Context, string, string, string) (productspublic.Product, error) {
	return productspublic.Product{}, nil
}

// fakeUsers knows user "u1" only.
type fakeUsers struct{}

func (fakeUsers) Create(context.Context, userspublic.CreateUserInput) (userspublic.User, error) {
	return userspublic.User{}, nil
}
func (fakeUsers) GetByID(_ context.Context, id string) (userspublic.User, error) {
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
func (fakeUsers) SetRole(_ context.Context, id string, r userspublic.Role) (userspublic.User, error) {
	return userspublic.User{ID: id, Role: r}, nil
}

func newService() public.Service {
	return service.NewService(newFakeRepo(), fakeProducts{}, fakeUsers{})
}

func TestAddAndMerge(t *testing.T) {
	ctx := context.Background()
	svc := newService()

	// Base product snapshots base price.
	cart, err := svc.AddItem(ctx, public.AddItemInput{UserID: "u1", ProductID: "prod-1", Qty: 1})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if len(cart.Items) != 1 || cart.Items[0].UnitPriceCents != 1000 || cart.SubtotalCents != 1000 {
		t.Fatalf("bad snapshot: %+v", cart)
	}

	// Same line merges qty instead of duplicating.
	cart, err = svc.AddItem(ctx, public.AddItemInput{UserID: "u1", ProductID: "prod-1", Qty: 2})
	if err != nil {
		t.Fatalf("re-add: %v", err)
	}
	if len(cart.Items) != 1 || cart.Items[0].Qty != 3 || cart.SubtotalCents != 3000 {
		t.Fatalf("merge failed: %+v", cart)
	}

	// Variant line snapshots the SKU price, separate line.
	variantID := "var-1"
	cart, err = svc.AddItem(ctx, public.AddItemInput{UserID: "u1", ProductID: "prod-1", VariantID: &variantID, Qty: 1})
	if err != nil {
		t.Fatalf("variant add: %v", err)
	}
	if len(cart.Items) != 2 || cart.SubtotalCents != 3000+2500 {
		t.Fatalf("variant line wrong: %+v", cart)
	}
	if cart.Items[1].SKU != "OIL-500" {
		t.Fatalf("sku not snapshotted: %+v", cart.Items[1])
	}
}

func TestGuardrails(t *testing.T) {
	ctx := context.Background()
	svc := newService()

	if _, err := svc.AddItem(ctx, public.AddItemInput{UserID: "u1", ProductID: "ghost", Qty: 1}); apperr.CodeOf(err) != apperr.CodeNotFound {
		t.Fatalf("unknown product: expected NOT_FOUND, got %v", err)
	}
	if _, err := svc.AddItem(ctx, public.AddItemInput{UserID: "u1", ProductID: "prod-off", Qty: 1}); apperr.CodeOf(err) != apperr.CodeConflict {
		t.Fatalf("inactive product: expected CONFLICT, got %v", err)
	}
	badVariant := "nope"
	if _, err := svc.AddItem(ctx, public.AddItemInput{UserID: "u1", ProductID: "prod-1", VariantID: &badVariant, Qty: 1}); apperr.CodeOf(err) != apperr.CodeNotFound {
		t.Fatalf("unknown variant: expected NOT_FOUND, got %v", err)
	}
	if _, err := svc.AddItem(ctx, public.AddItemInput{UserID: "ghost", ProductID: "prod-1", Qty: 1}); apperr.CodeOf(err) != apperr.CodeNotFound {
		t.Fatalf("unknown user: expected NOT_FOUND, got %v", err)
	}
	if _, err := svc.AddItem(ctx, public.AddItemInput{UserID: "u1", ProductID: "prod-1", Qty: 0}); apperr.CodeOf(err) != apperr.CodeValidation {
		t.Fatalf("zero qty: expected VALIDATION_ERROR, got %v", err)
	}
}

func TestQtyLifecycle(t *testing.T) {
	ctx := context.Background()
	svc := newService()

	cart, err := svc.AddItem(ctx, public.AddItemInput{UserID: "u1", ProductID: "prod-1", Qty: 2})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	itemID := cart.Items[0].ID

	cart, err = svc.SetQty(ctx, "u1", itemID, 5)
	if err != nil || cart.Items[0].Qty != 5 || cart.SubtotalCents != 5000 {
		t.Fatalf("set qty: %v %+v", err, cart)
	}
	// Qty 0 removes the line.
	cart, err = svc.SetQty(ctx, "u1", itemID, 0)
	if err != nil || len(cart.Items) != 0 || cart.SubtotalCents != 0 {
		t.Fatalf("zero removes: %v %+v", err, cart)
	}
	if _, err := svc.SetQty(ctx, "u1", itemID, 2); apperr.CodeOf(err) != apperr.CodeNotFound {
		t.Fatalf("expected NOT_FOUND, got %v", err)
	}
}
