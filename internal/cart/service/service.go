package service

import (
	"context"
	"strings"

	"github.com/nikhea/malawi-e-commerce-store/internal/cart/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/cart/public"
	productspublic "github.com/nikhea/malawi-e-commerce-store/internal/products/public"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	"github.com/nikhea/malawi-e-commerce-store/pkg/bulk"
	bulkdto "github.com/nikhea/malawi-e-commerce-store/pkg/bulk/dto"
	"github.com/nikhea/malawi-e-commerce-store/pkg/pagination"
)

var _ public.Service = (*service)(nil)

// Repository is the port this service needs from persistence.
type Repository interface {
	GetOrCreateCart(ctx context.Context, userID string) (model.Cart, error)
	ListItems(ctx context.Context, cartID string) ([]model.Item, error)
	ListItemsPage(ctx context.Context, cartID string, f model.ListFilter) ([]model.Item, int64, error)
	Totals(ctx context.Context, cartID string) (int64, string, error)
	UpsertItem(ctx context.Context, i model.Item) (model.Item, error)
	UpsertMany(ctx context.Context, items []model.Item) ([]model.Item, int, error)
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
	var subtotal int64
	currency := "USD"
	for _, i := range items {
		subtotal += i.UnitPriceCents * int64(i.Qty)
		currency = i.Currency
	}
	return toPublicCart(c, items, subtotal, currency)
}

func toPublicLine(i model.Item) public.Line {
	return public.Line{
		ID: i.ID, ProductID: i.ProductID, VariantID: i.VariantID,
		Name: i.Name, SKU: i.SKU, UnitPriceCents: i.UnitPriceCents,
		Currency: i.Currency, Qty: i.Qty,
		LineTotalCents: i.UnitPriceCents * int64(i.Qty),
	}
}

func toPublicCart(c model.Cart, items []model.Item, subtotal int64, currency string) public.Cart {
	lines := make([]public.Line, 0, len(items))
	for _, i := range items {
		lines = append(lines, toPublicLine(i))
	}
	if currency == "" {
		currency = "USD"
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

var cartSorts = map[string]bool{
	"oldest":     true,
	"newest":     true,
	"name_asc":   true,
	"name_desc":  true,
	"price_asc":  true,
	"price_desc": true,
	"qty_asc":    true,
	"qty_desc":   true,
}

// List serves the paged GET /cart. Totals stay whole-cart totals so the
// client's checkout summary never changes while paging through lines.
func (s *service) List(ctx context.Context, userID string, f public.ListFilter) (public.Page, error) {
	if f.Limit < 0 {
		return public.Page{}, apperr.Validation("limit cannot be negative")
	}
	if f.Offset < 0 {
		return public.Page{}, apperr.Validation("offset cannot be negative")
	}
	if f.Page < 0 {
		return public.Page{}, apperr.Validation("page cannot be negative")
	}
	sortKey := strings.TrimSpace(f.Sort)
	if sortKey == "" {
		sortKey = "oldest"
	}
	if !cartSorts[sortKey] {
		return public.Page{}, apperr.Validation("unknown sort; use oldest, newest, name_asc, name_desc, price_asc, price_desc, qty_asc or qty_desc")
	}
	if strings.TrimSpace(userID) == "" {
		return public.Page{}, apperr.Validation("user id is required")
	}
	if _, err := s.users.GetByID(ctx, userID); err != nil {
		return public.Page{}, err
	}
	cart, err := s.repo.GetOrCreateCart(ctx, userID)
	if err != nil {
		return public.Page{}, err
	}
	limit, offset := pagination.NormalizeWithDefault(f.Page, f.Limit, f.Offset, pagination.DefaultListLimit)
	items, total, err := s.repo.ListItemsPage(ctx, cart.ID, model.ListFilter{
		Query: strings.TrimSpace(f.Query), ProductID: trimPtr(f.ProductID),
		VariantID: trimPtr(f.VariantID), Sort: sortKey, Limit: limit, Offset: offset,
	})
	if err != nil {
		return public.Page{}, err
	}
	subtotal, currency, err := s.repo.Totals(ctx, cart.ID)
	if err != nil {
		return public.Page{}, err
	}
	return public.Page{
		Cart: toPublicCart(cart, items, subtotal, currency),
		Meta: pagination.NewMeta(total, limit, offset),
	}, nil
}

func trimPtr(v *string) *string {
	if v == nil || strings.TrimSpace(*v) == "" {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	return &trimmed
}

// prepareAdd validates one line against the catalog and snapshots its
// price. It does no writes: single add, bulk add and their per-item error
// reporting share this exact semantics. CartID is filled by the caller.
func (s *service) prepareAdd(ctx context.Context, productID string, variantID *string, qty int) (model.Item, error) {
	if qty <= 0 {
		return model.Item{}, apperr.Validation("qty must be positive")
	}
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return model.Item{}, apperr.Validation("product id is required")
	}

	// Catalog validation + price snapshot in one detail call.
	detail, err := s.products.GetByID(ctx, productID)
	if err != nil {
		return model.Item{}, err // product NOT_FOUND passes through
	}
	if !detail.Product.IsActive {
		return model.Item{}, apperr.Conflict("product is not available")
	}
	// Stock is tracked per variant, not on the base product, so a
	// variant-less line has nothing to reserve at checkout. Require the
	// SKU whenever the product has one; base-price lines stay valid for
	// products that genuinely have no variants.
	if variantID == nil && len(detail.Variants) > 0 {
		return model.Item{}, apperr.Validation("variant id is required for this product")
	}
	name, sku, price := detail.Product.Name, "", detail.Product.PriceCents
	if variantID != nil {
		v, ok := findVariant(detail.Variants, *variantID)
		if !ok {
			return model.Item{}, apperr.NotFound("variant not found")
		}
		if !v.IsActive {
			return model.Item{}, apperr.Conflict("variant is not available")
		}
		name, sku, price = detail.Product.Name+" / "+v.Name, v.SKU, v.PriceCents
	}
	return model.Item{
		ProductID: productID, VariantID: variantID,
		Name: name, SKU: sku, UnitPriceCents: price,
		Currency: detail.Product.Currency, Qty: qty,
	}, nil
}

func (s *service) AddItem(ctx context.Context, in public.AddItemInput) (public.Cart, error) {
	item, err := s.prepareAdd(ctx, in.ProductID, in.VariantID, in.Qty)
	if err != nil {
		return public.Cart{}, err
	}
	cart, _, err := s.load(ctx, in.UserID)
	if err != nil {
		return public.Cart{}, err
	}
	item.CartID = cart.ID
	if _, err := s.repo.UpsertItem(ctx, item); err != nil {
		return public.Cart{}, err
	}
	if err := s.repo.Touch(ctx, cart.ID); err != nil {
		return public.Cart{}, apperr.Internal(err)
	}
	_, view, err := s.load(ctx, in.UserID)
	return view, err
}

// BulkAdd adds up to 25 lines for the JWT caller. Identity never comes from
// the body. Non-atomic batches keep going after a bad line; atomic batches
// validate everything first and write through one transaction.
func (s *service) BulkAdd(ctx context.Context, userID string, items []public.BulkAddItem, atomic bool) ([]bulkdto.BulkResult[public.Line], public.Cart, error) {
	limit := 25
	if max := bulk.MaxItems(); max < limit {
		limit = max
	}
	if err := bulk.CheckCount(len(items), limit); err != nil {
		return nil, public.Cart{}, err
	}

	keys := make([]string, 0, len(items))
	for _, in := range items {
		productID := strings.TrimSpace(in.ProductID)
		if productID == "" {
			continue
		}
		keys = append(keys, bulk.LineKey(productID, in.VariantID))
	}
	if dup, ok := bulk.FindDuplicate(keys); ok {
		return nil, public.Cart{}, apperr.Duplicate("duplicate cart line: " + dup)
	}

	cart, before, err := s.load(ctx, userID)
	if err != nil {
		return nil, public.Cart{}, err
	}

	prepared := make([]model.Item, len(items))
	itemErrs := make([]error, len(items))
	results := make([]bulkdto.BulkResult[public.Line], len(items))
	for i, in := range items {
		key := bulk.LineKey(strings.TrimSpace(in.ProductID), in.VariantID)
		results[i] = bulkdto.BulkResult[public.Line]{Index: i, ID: key, Status: "ok"}
		item, err := s.prepareAdd(ctx, in.ProductID, in.VariantID, in.Qty)
		if err != nil {
			itemErrs[i] = err
			results[i] = bulk.Failed[public.Line](i, key, err)
			continue
		}
		item.CartID = cart.ID
		prepared[i] = item
	}

	if atomic {
		for i, err := range itemErrs {
			if err != nil {
				return bulk.Rollback(results, i, err), before, nil
			}
		}
		rows, failIndex, err := s.repo.UpsertMany(ctx, prepared)
		if err != nil {
			if failIndex < 0 {
				failIndex = 0
			}
			return bulk.Rollback(results, failIndex, err), before, nil
		}
		for i := range items {
			results[i] = bulk.OK(i, rows[i].ID, toPublicLine(rows[i]))
		}
	} else {
		wrote := false
		for i := range items {
			if itemErrs[i] != nil {
				continue
			}
			row, err := s.repo.UpsertItem(ctx, prepared[i])
			if err != nil {
				results[i] = bulk.Failed[public.Line](i, results[i].ID, err)
				continue
			}
			wrote = true
			results[i] = bulk.OK(i, row.ID, toPublicLine(row))
		}
		if wrote {
			if err := s.repo.Touch(ctx, cart.ID); err != nil {
				return nil, public.Cart{}, apperr.Internal(err)
			}
		}
	}

	_, view, err := s.load(ctx, userID)
	if err != nil {
		return nil, public.Cart{}, err
	}
	return results, view, nil
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
