package service

import (
	"context"
	"strings"

	"github.com/nikhea/malawi-e-commerce-store/internal/cart/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/cart/public"
	productspublic "github.com/nikhea/malawi-e-commerce-store/internal/products/public"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
)

var _ public.Service = (*service)(nil)

// Repository is the port this service needs from persistence.
type Repository interface {
	GetOrCreateCart(ctx context.Context, userID string) (model.Cart, error)
	ListItems(ctx context.Context, cartID string) ([]model.Item, error)
	UpsertItem(ctx context.Context, i model.Item) (model.Item, error)
	GetItem(ctx context.Context, cartID, itemID string) (model.Item, error)
	SetQty(ctx context.Context, cartID, itemID string, qty int) (model.Item, error)
	DeleteItem(ctx context.Context, cartID, itemID string) error
	Clear(ctx context.Context, cartID string) error
	Touch(ctx context.Context, cartID string) error
}

type service struct {
	repo     Repository
	products productspublic.Service
	users    userspublic.Service
}

func NewService(repo Repository, products productspublic.Service, users userspublic.Service) public.Service {
	return &service{repo: repo, products: products, users: users}
}

func toPublic(c model.Cart, items []model.Item) public.Cart {
	lines := make([]public.Line, 0, len(items))
	var subtotal int64
	currency := "MWK"
	for _, i := range items {
		total := i.UnitPriceCents * int64(i.Qty)
		subtotal += total
		currency = i.Currency
		lines = append(lines, public.Line{
			ID: i.ID, ProductID: i.ProductID, VariantID: i.VariantID,
			Name: i.Name, SKU: i.SKU, UnitPriceCents: i.UnitPriceCents,
			Currency: i.Currency, Qty: i.Qty, LineTotalCents: total,
		})
	}
	return public.Cart{
		ID: c.ID, UserID: c.UserID, Items: lines,
		SubtotalCents: subtotal, Currency: currency, UpdatedAt: c.UpdatedAt,
	}
}

// load returns the cart with lines, verifying the owner exists first so
// deleted users can't keep shopping on orphaned carts.
func (s *service) load(ctx context.Context, userID string) (model.Cart, public.Cart, error) {
	if strings.TrimSpace(userID) == "" {
		return model.Cart{}, public.Cart{}, apperr.Validation("user id is required")
	}
	if _, err := s.users.GetByID(ctx, userID); err != nil {
		return model.Cart{}, public.Cart{}, err
	}
	cart, err := s.repo.GetOrCreateCart(ctx, userID)
	if err != nil {
		return model.Cart{}, public.Cart{}, err
	}
	items, err := s.repo.ListItems(ctx, cart.ID)
	if err != nil {
		return model.Cart{}, public.Cart{}, err
	}
	return cart, toPublic(cart, items), nil
}

func (s *service) Get(ctx context.Context, userID string) (public.Cart, error) {
	_, cart, err := s.load(ctx, userID)
	return cart, err
}

func (s *service) AddItem(ctx context.Context, in public.AddItemInput) (public.Cart, error) {
	if in.Qty <= 0 {
		return public.Cart{}, apperr.Validation("qty must be positive")
	}
	if strings.TrimSpace(in.ProductID) == "" {
		return public.Cart{}, apperr.Validation("product id is required")
	}

	// Catalog validation + price snapshot in one detail call.
	detail, err := s.products.GetByID(ctx, in.ProductID)
	if err != nil {
		return public.Cart{}, err // product NOT_FOUND passes through
	}
	if !detail.Product.IsActive {
		return public.Cart{}, apperr.Conflict("product is not available")
	}
	name, sku, price := detail.Product.Name, "", detail.Product.PriceCents
	if in.VariantID != nil {
		v, ok := findVariant(detail.Variants, *in.VariantID)
		if !ok {
			return public.Cart{}, apperr.NotFound("variant not found")
		}
		if !v.IsActive {
			return public.Cart{}, apperr.Conflict("variant is not available")
		}
		name, sku, price = detail.Product.Name+" / "+v.Name, v.SKU, v.PriceCents
	}

	cart, _, err := s.load(ctx, in.UserID)
	if err != nil {
		return public.Cart{}, err
	}
	if _, err := s.repo.UpsertItem(ctx, model.Item{
		CartID: cart.ID, ProductID: in.ProductID, VariantID: in.VariantID,
		Name: name, SKU: sku, UnitPriceCents: price,
		Currency: detail.Product.Currency, Qty: in.Qty,
	}); err != nil {
		return public.Cart{}, err
	}
	if err := s.repo.Touch(ctx, cart.ID); err != nil {
		return public.Cart{}, apperr.Internal(err)
	}
	_, view, err := s.load(ctx, in.UserID)
	return view, err
}

func findVariant(variants []productspublic.VariantView, id string) (productspublic.VariantView, bool) {
	for _, v := range variants {
		if v.ID == id {
			return v, true
		}
	}
	return productspublic.VariantView{}, false
}

func (s *service) SetQty(ctx context.Context, userID, itemID string, qty int) (public.Cart, error) {
	if qty < 0 {
		return public.Cart{}, apperr.Validation("qty cannot be negative")
	}
	cart, _, err := s.load(ctx, userID)
	if err != nil {
		return public.Cart{}, err
	}
	if qty == 0 {
		if err := s.repo.DeleteItem(ctx, cart.ID, itemID); err != nil {
			return public.Cart{}, err
		}
	} else {
		if _, err := s.repo.GetItem(ctx, cart.ID, itemID); err != nil {
			return public.Cart{}, err // NOT_FOUND passes through
		}
		if _, err := s.repo.SetQty(ctx, cart.ID, itemID, qty); err != nil {
			return public.Cart{}, err
		}
	}
	if err := s.repo.Touch(ctx, cart.ID); err != nil {
		return public.Cart{}, apperr.Internal(err)
	}
	_, view, err := s.load(ctx, userID)
	return view, err
}

func (s *service) RemoveItem(ctx context.Context, userID, itemID string) (public.Cart, error) {
	cart, _, err := s.load(ctx, userID)
	if err != nil {
		return public.Cart{}, err
	}
	if err := s.repo.DeleteItem(ctx, cart.ID, itemID); err != nil {
		return public.Cart{}, err
	}
	if err := s.repo.Touch(ctx, cart.ID); err != nil {
		return public.Cart{}, apperr.Internal(err)
	}
	_, view, err := s.load(ctx, userID)
	return view, err
}

func (s *service) Clear(ctx context.Context, userID string) error {
	cart, _, err := s.load(ctx, userID)
	if err != nil {
		return err
	}
	return s.repo.Clear(ctx, cart.ID)
}
