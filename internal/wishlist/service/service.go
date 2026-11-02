package service

import (
	"context"
	"strings"

	productspublic "github.com/nikhea/malawi-e-commerce-store/internal/products/public"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/wishlist/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/wishlist/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	"github.com/nikhea/malawi-e-commerce-store/pkg/bulk"
	bulkdto "github.com/nikhea/malawi-e-commerce-store/pkg/bulk/dto"
	"github.com/nikhea/malawi-e-commerce-store/pkg/pagination"
)

var _ public.Service = (*service)(nil)

// Repository is the port this service needs from persistence.
type Repository interface {
	// Add reports whether the link was inserted (created).
	Add(ctx context.Context, userID, productID string) (model.Item, bool, error)
	AddMany(ctx context.Context, userID string, productIDs []string) ([]model.AddResult, int, error)
	Existing(ctx context.Context, userID string, productIDs []string) (map[string]bool, error)
	ListByUser(ctx context.Context, userID string, f model.ListFilter) ([]model.Item, int64, error)
	Remove(ctx context.Context, userID, productID string) error
	RemoveMany(ctx context.Context, userID string, productIDs []string) (int, error)
}

type service struct {
	repo     Repository
	products productspublic.Service
	users    userspublic.Service
}

func NewService(repo Repository, products productspublic.Service, users userspublic.Service) public.Service {
	return &service{repo: repo, products: products, users: users}
}

func toPublicItem(detail productspublic.Detail, i model.Item) public.Item {
	p := detail.Product
	return public.Item{
		ProductID: i.ProductID,
		Product: public.ProductSummary{
			ID: p.ID, Name: p.Name, Slug: p.Slug,
			PriceCents: p.PriceCents, Currency: p.Currency, ImageURL: p.ImageURL,
		},
		AddedAt: i.CreatedAt,
	}
}

func (s *service) attach(ctx context.Context, i model.Item) (public.Item, error) {
	detail, err := s.products.GetByID(ctx, i.ProductID)
	if err != nil {
		return public.Item{}, err
	}
	return toPublicItem(detail, i), nil
}

func (s *service) Add(ctx context.Context, userID, productID string) (public.Item, error) {
	if strings.TrimSpace(productID) == "" {
		return public.Item{}, apperr.Validation("product id is required")
	}
	if _, err := s.users.GetByID(ctx, userID); err != nil {
		return public.Item{}, err
	}
	// Existence check through the catalog (also fails deleted products).
	if _, err := s.products.GetByID(ctx, productID); err != nil {
		return public.Item{}, err
	}
	item, _, err := s.repo.Add(ctx, userID, productID)
	if err != nil {
		return public.Item{}, err
	}
	return s.attach(ctx, item)
}

func (s *service) Remove(ctx context.Context, userID, productID string) error {
	if strings.TrimSpace(productID) == "" {
		return apperr.Validation("product id is required")
	}
	return s.repo.Remove(ctx, userID, productID)
}

var wishlistSorts = map[string]bool{
	"newest":     true,
	"oldest":     true,
	"name_asc":   true,
	"name_desc":  true,
	"price_asc":  true,
	"price_desc": true,
}

func (s *service) ListMine(ctx context.Context, userID string, f public.ListFilter) (public.Page, error) {
	if strings.TrimSpace(userID) == "" {
		return public.Page{}, apperr.Validation("user id is required")
	}
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
		sortKey = "newest"
	}
	if !wishlistSorts[sortKey] {
		return public.Page{}, apperr.Validation("unknown sort; use newest, oldest, name_asc, name_desc, price_asc or price_desc")
	}
	limit, offset := pagination.NormalizeWithDefault(f.Page, f.Limit, f.Offset, pagination.DefaultListLimit)
	items, total, err := s.repo.ListByUser(ctx, userID, model.ListFilter{
		Query: strings.TrimSpace(f.Query), CategoryID: trimPtr(f.CategoryID),
		Sort: sortKey, Limit: limit, Offset: offset,
	})
	if err != nil {
		return public.Page{}, err
	}
	out := make([]public.Item, 0, len(items))
	for _, i := range items {
		attached, err := s.attach(ctx, i)
		if err != nil {
			// Product deleted between page and attach (FK cascade will
			// clean the link): skip instead of failing the whole page.
			continue
		}
		out = append(out, attached)
	}
	return public.Page{Items: out, Meta: pagination.NewMeta(total, limit, offset)}, nil
}

func trimPtr(v *string) *string {
	if v == nil || strings.TrimSpace(*v) == "" {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	return &trimmed
}

// normalizeIDs trims every id, rejects duplicates inside one request, and
// returns the ids plus the non-empty subset used for bulk queries.
func normalizeIDs(productIDs []string, label string) ([]string, []string, error) {
	ids := make([]string, len(productIDs))
	dupeKeys := make([]string, 0, len(productIDs))
	valid := make([]string, 0, len(productIDs))
	for i, raw := range productIDs {
		ids[i] = strings.TrimSpace(raw)
		if ids[i] == "" {
			continue
		}
		dupeKeys = append(dupeKeys, ids[i])
		valid = append(valid, ids[i])
	}
	if dup, ok := bulk.FindDuplicate(dupeKeys); ok {
		return nil, nil, apperr.Duplicate("duplicate " + label + " id: " + dup)
	}
	return ids, valid, nil
}

// BulkAdd hearts up to MAX_BULK_ITEMS products. Unknown or inactive products
// are per-item NOT_FOUND; already-hearted links are success with
// AlreadyExists. atomic inserts every new link in one transaction.
func (s *service) BulkAdd(ctx context.Context, userID string, productIDs []string, atomic bool) ([]bulkdto.BulkResult[public.Item], error) {
	if err := bulk.CheckCount(len(productIDs), bulk.MaxItems()); err != nil {
		return nil, err
	}
	ids, _, err := normalizeIDs(productIDs, "product")
	if err != nil {
		return nil, err
	}
	if _, err := s.users.GetByID(ctx, userID); err != nil {
		return nil, err
	}

	details := make([]productspublic.Detail, len(ids))
	itemErrs := make([]error, len(ids))
	results := make([]bulkdto.BulkResult[public.Item], len(ids))
	for i, id := range ids {
		results[i] = bulkdto.BulkResult[public.Item]{Index: i, ID: id, Status: "ok"}
		if id == "" {
			itemErrs[i] = apperr.Validation("product id is required")
			results[i] = bulk.Failed[public.Item](i, id, itemErrs[i])
			continue
		}
		detail, err := s.products.GetByID(ctx, id)
		if err != nil {
			itemErrs[i] = err
			results[i] = bulk.Failed[public.Item](i, id, err)
			continue
		}
		if !detail.Product.IsActive {
			itemErrs[i] = apperr.NotFound("product not found")
			results[i] = bulk.Failed[public.Item](i, id, itemErrs[i])
			continue
		}
		details[i] = detail
	}

	if atomic {
		for i, err := range itemErrs {
			if err != nil {
				return bulk.Rollback(results, i, err), nil
			}
		}
		added, failIndex, err := s.repo.AddMany(ctx, userID, ids)
		if err != nil {
			if failIndex < 0 {
				failIndex = 0
			}
			return bulk.Rollback(results, failIndex, err), nil
		}
		for i := range ids {
			results[i] = bulk.OK(i, ids[i], toPublicItem(details[i], added[i].Item))
			results[i].AlreadyExists = !added[i].Created
		}
		return results, nil
	}

	for i, id := range ids {
		if itemErrs[i] != nil {
			continue
		}
		item, created, err := s.repo.Add(ctx, userID, id)
		if err != nil {
			results[i] = bulk.Failed[public.Item](i, id, err)
			continue
		}
		results[i] = bulk.OK(i, id, toPublicItem(details[i], item))
		results[i].AlreadyExists = !created
	}
	return results, nil
}

// BulkRemove unhearts by product id (matching single DELETE /wishlist/{id}).
// Unknown products are per-item NOT_FOUND; known-but-not-hearted links are
// success with AlreadyRemoved. dryRun reports WouldDelete without writing.
func (s *service) BulkRemove(ctx context.Context, userID string, productIDs []string, atomic, dryRun bool) ([]bulkdto.BulkResult[bulkdto.Empty], error) {
	if err := bulk.CheckCount(len(productIDs), bulk.MaxItems()); err != nil {
		return nil, err
	}
	ids, valid, err := normalizeIDs(productIDs, "product")
	if err != nil {
		return nil, err
	}
	if _, err := s.users.GetByID(ctx, userID); err != nil {
		return nil, err
	}
	existing, err := s.repo.Existing(ctx, userID, valid)
	if err != nil {
		return nil, err
	}

	itemErrs := make([]error, len(ids))
	results := make([]bulkdto.BulkResult[bulkdto.Empty], len(ids))
	toRemove := make([]string, 0, len(ids))
	for i, id := range ids {
		results[i] = bulk.OK[bulkdto.Empty](i, id, bulkdto.Empty{})
		if id == "" {
			itemErrs[i] = apperr.Validation("product id is required")
			results[i] = bulk.Failed[bulkdto.Empty](i, id, itemErrs[i])
			continue
		}
		if _, err := s.products.GetByID(ctx, id); err != nil {
			itemErrs[i] = err
			results[i] = bulk.Failed[bulkdto.Empty](i, id, err)
			continue
		}
		if existing[id] {
			toRemove = append(toRemove, id)
		}
	}

	if dryRun {
		for i, id := range ids {
			if itemErrs[i] != nil {
				continue
			}
			if existing[id] {
				results[i].WouldDelete = true
			} else {
				results[i].AlreadyRemoved = true
			}
		}
		return results, nil
	}

	if atomic {
		for i, err := range itemErrs {
			if err != nil {
				return bulk.Rollback(results, i, err), nil
			}
		}
		if failIndex, err := s.repo.RemoveMany(ctx, userID, toRemove); err != nil {
			if failIndex < 0 {
				failIndex = 0
			}
			return bulk.Rollback(results, failIndex, err), nil
		}
		for i, id := range ids {
			if !existing[id] {
				results[i].AlreadyRemoved = true
			}
		}
		return results, nil
	}

	for i, id := range ids {
		if itemErrs[i] != nil {
			continue
		}
		if !existing[id] {
			results[i].AlreadyRemoved = true
			continue
		}
		if err := s.repo.Remove(ctx, userID, id); err != nil {
			if apperr.CodeOf(err) == apperr.CodeNotFound {
				results[i].AlreadyRemoved = true
				continue
			}
			results[i] = bulk.Failed[bulkdto.Empty](i, id, err)
			continue
		}
		results[i] = bulk.OK[bulkdto.Empty](i, id, bulkdto.Empty{})
	}
	return results, nil
}
