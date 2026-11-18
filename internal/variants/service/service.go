package service

import (
	"context"
	"strings"

	inventorypublic "github.com/nikhea/malawi-e-commerce-store/internal/inventory/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/variants/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/variants/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	"github.com/nikhea/malawi-e-commerce-store/pkg/bulk"
)

var _ public.Service = (*service)(nil)

// Repository is the port this service needs from persistence.
type Repository interface {
	Create(ctx context.Context, v model.Variant) (model.Variant, error)
	CreateMany(ctx context.Context, items []model.Variant) ([]model.Variant, int, error)
	GetByID(ctx context.Context, id string) (model.Variant, error)
	ListByProduct(ctx context.Context, productID string) ([]model.Variant, error)
	Update(ctx context.Context, v model.Variant) (model.Variant, error)
	UpdateMany(ctx context.Context, items []model.Variant) ([]model.Variant, int, error)
	Delete(ctx context.Context, id string) error
	ProductExists(ctx context.Context, id string) (bool, error)
}

type service struct {
	repo      Repository
	inventory inventorypublic.Service
}

func NewService(repo Repository, inventory inventorypublic.Service) public.Service {
	return &service{repo: repo, inventory: inventory}
}

func toPublic(v model.Variant, available int) public.Variant {
	return public.Variant{
		ID: v.ID, ProductID: v.ProductID, Name: v.Name, SKU: v.SKU,
		PriceCents: v.PriceCents, IsActive: v.IsActive,
		Available: available, InStock: available > 0,
		CreatedAt: v.CreatedAt,
	}
}

// prepareCreate validates and normalizes one SKU. Single and bulk create
// share it so their rules can never drift.
func prepareCreate(in public.CreateVariantInput) (model.Variant, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return model.Variant{}, apperr.Validation("name is required")
	}
	sku := strings.ToUpper(strings.TrimSpace(in.SKU))
	if sku == "" {
		return model.Variant{}, apperr.Validation("sku is required")
	}
	if in.PriceCents < 0 {
		return model.Variant{}, apperr.Validation("price cannot be negative")
	}
	return model.Variant{
		ProductID: in.ProductID, Name: name, SKU: sku, PriceCents: in.PriceCents,
	}, nil
}

func (s *service) Create(ctx context.Context, in public.CreateVariantInput) (public.Variant, error) {
	if strings.TrimSpace(in.ProductID) == "" {
		return public.Variant{}, apperr.Validation("product id is required")
	}
	prepared, err := prepareCreate(in)
	if err != nil {
		return public.Variant{}, err
	}

	v, err := s.repo.Create(ctx, prepared)
	if err != nil {
		return public.Variant{}, err // CONFLICT / product NOT_FOUND pass through
	}
	// A new SKU has no inventory row yet: available is genuinely zero.
	return toPublic(v, 0), nil
}

func (s *service) ListByProduct(ctx context.Context, productID string) ([]public.Variant, error) {
	if strings.TrimSpace(productID) == "" {
		return nil, apperr.Validation("product id is required")
	}
	variants, err := s.repo.ListByProduct(ctx, productID)
	if err != nil {
		return nil, err
	}
	stocks, err := s.inventory.ListStockByProduct(ctx, productID)
	if err != nil {
		return nil, err
	}
	available := make(map[string]int, len(stocks))
	for _, stock := range stocks {
		if stock.VariantID != nil {
			available[*stock.VariantID] = stock.Available
		}
	}
	out := make([]public.Variant, 0, len(variants))
	for _, v := range variants {
		out = append(out, toPublic(v, available[v.ID]))
	}
	return out, nil
}

// applyUpdate projects a patch onto the current row. It performs no writes:
// single update, bulk update and dry runs share this exact semantics.
func applyUpdate(current model.Variant, in public.UpdateVariantInput) (model.Variant, error) {
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return model.Variant{}, apperr.Validation("name is required")
		}
		current.Name = name
	}
	if in.PriceCents != nil {
		if *in.PriceCents < 0 {
			return model.Variant{}, apperr.Validation("price cannot be negative")
		}
		current.PriceCents = *in.PriceCents
	}
	if in.IsActive != nil {
		current.IsActive = *in.IsActive
	}
	// NOTE: product_id and sku are immutable (order history references
	// them); changing either means creating a new variant.
	return current, nil
}

func isEmptyPatch(in public.UpdateVariantInput) bool {
	return in.Name == nil && in.PriceCents == nil && in.IsActive == nil
}

func (s *service) Update(ctx context.Context, id string, in public.UpdateVariantInput) (public.Variant, error) {
	if strings.TrimSpace(id) == "" {
		return public.Variant{}, apperr.Validation("id is required")
	}
	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return public.Variant{}, err
	}
	projected, err := applyUpdate(current, in)
	if err != nil {
		return public.Variant{}, err
	}

	// Read stock before the write so a stock lookup failure cannot leave
	// the caller with a 500 after the variant was already updated.
	stock, err := s.inventory.GetStock(ctx, projected.ProductID, &projected.ID)
	if err != nil {
		return public.Variant{}, err
	}
	updated, err := s.repo.Update(ctx, projected)
	if err != nil {
		return public.Variant{}, err
	}
	return toPublic(updated, stock.Available), nil
}

func (s *service) Delete(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return apperr.Validation("id is required")
	}
	return s.repo.Delete(ctx, id)
}

// BulkCreate creates up to MAX_BULK_ITEMS SKUs for one product. The product
// existence check is whole-request (404) so a bad path never produces a
// half-applied batch. Duplicate SKUs inside the batch reject the request;
// a DB-level SKU collision is per item.
func (s *service) BulkCreate(ctx context.Context, productID string, items []public.CreateVariantInput, atomic, dryRun bool) ([]bulk.BulkResult[public.Variant], error) {
	if err := bulk.CheckCount(len(items), bulk.MaxItems()); err != nil {
		return nil, err
	}
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return nil, apperr.Validation("product id is required")
	}
	exists, err := s.repo.ProductExists(ctx, productID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, apperr.NotFound("product not found")
	}

	prepared := make([]model.Variant, len(items))
	prepErrs := make([]error, len(items))
	skus := make([]string, 0, len(items))
	for i, in := range items {
		in.ProductID = productID
		v, err := prepareCreate(in)
		prepared[i], prepErrs[i] = v, err
		if err == nil {
			skus = append(skus, v.SKU)
		}
	}
	if dup, ok := bulk.FindDuplicate(skus); ok {
		return nil, apperr.Duplicate("duplicate variant sku: " + dup)
	}

	results := make([]bulk.BulkResult[public.Variant], len(items))
	for i := range items {
		results[i] = bulk.BulkResult[public.Variant]{Index: i, ID: prepared[i].SKU, Status: "ok"}
		if prepErrs[i] != nil {
			results[i] = bulk.Failed[public.Variant](i, prepared[i].SKU, prepErrs[i])
		}
	}

	if dryRun {
		for i := range items {
			if prepErrs[i] == nil {
				results[i] = bulk.OK(i, prepared[i].SKU, toPublic(prepared[i], 0))
			}
		}
		return results, nil
	}

	if atomic {
		for i, err := range prepErrs {
			if err != nil {
				return bulk.Rollback(results, i, err), nil
			}
		}
		created, failIndex, err := s.repo.CreateMany(ctx, prepared)
		if err != nil {
			if failIndex < 0 {
				failIndex = 0
			}
			return bulk.Rollback(results, failIndex, err), nil
		}
		for i := range items {
			results[i] = bulk.OK(i, created[i].SKU, toPublic(created[i], 0))
		}
		return results, nil
	}

	for i := range items {
		if prepErrs[i] != nil {
			continue
		}
		v, err := s.repo.Create(ctx, prepared[i])
		if err != nil {
			results[i] = bulk.Failed[public.Variant](i, prepared[i].SKU, err)
			continue
		}
		results[i] = bulk.OK(i, v.SKU, toPublic(v, 0))
	}
	return results, nil
}

// BulkUpdate applies one partial patch per variant. Empty patches are
// per-item validation errors; duplicate ids reject the whole batch. Stock is
// read for every item before any write, so an inventory failure cannot leave
// a committed write behind a 500.
func (s *service) BulkUpdate(ctx context.Context, items []public.BulkUpdateItem, atomic, dryRun bool) ([]bulk.BulkResult[public.Variant], error) {
	if err := bulk.CheckCount(len(items), bulk.MaxItems()); err != nil {
		return nil, err
	}
	ids := make([]string, len(items))
	dupeKeys := make([]string, 0, len(items))
	for i, it := range items {
		ids[i] = strings.TrimSpace(it.ID)
		if ids[i] != "" {
			dupeKeys = append(dupeKeys, ids[i])
		}
	}
	if dup, ok := bulk.FindDuplicate(dupeKeys); ok {
		return nil, apperr.Duplicate("duplicate variant id: " + dup)
	}

	projected := make([]model.Variant, len(items))
	available := make([]int, len(items))
	itemErrs := make([]error, len(items))
	results := make([]bulk.BulkResult[public.Variant], len(items))
	for i, it := range items {
		results[i] = bulk.BulkResult[public.Variant]{Index: i, ID: ids[i], Status: "ok"}
		if ids[i] == "" {
			itemErrs[i] = apperr.Validation("id is required")
			results[i] = bulk.Failed[public.Variant](i, ids[i], itemErrs[i])
			continue
		}
		if isEmptyPatch(it.Patch) {
			itemErrs[i] = apperr.Validation("at least one field is required")
			results[i] = bulk.Failed[public.Variant](i, ids[i], itemErrs[i])
			continue
		}
		current, err := s.repo.GetByID(ctx, ids[i])
		if err != nil {
			itemErrs[i] = err
			results[i] = bulk.Failed[public.Variant](i, ids[i], err)
			continue
		}
		row, err := applyUpdate(current, it.Patch)
		if err != nil {
			itemErrs[i] = err
			results[i] = bulk.Failed[public.Variant](i, ids[i], err)
			continue
		}
		stock, err := s.inventory.GetStock(ctx, row.ProductID, &row.ID)
		if err != nil {
			itemErrs[i] = err
			results[i] = bulk.Failed[public.Variant](i, ids[i], err)
			continue
		}
		projected[i], available[i] = row, stock.Available
	}

	if dryRun {
		for i := range items {
			if itemErrs[i] == nil {
				results[i] = bulk.OK(i, ids[i], toPublic(projected[i], available[i]))
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
		rows, failIndex, err := s.repo.UpdateMany(ctx, projected)
		if err != nil {
			if failIndex < 0 {
				failIndex = 0
			}
			return bulk.Rollback(results, failIndex, err), nil
		}
		for i := range items {
			results[i] = bulk.OK(i, ids[i], toPublic(rows[i], available[i]))
		}
		return results, nil
	}

	for i := range items {
		if itemErrs[i] != nil {
			continue
		}
		row, err := s.repo.Update(ctx, projected[i])
		if err != nil {
			results[i] = bulk.Failed[public.Variant](i, ids[i], err)
			continue
		}
		results[i] = bulk.OK(i, ids[i], toPublic(row, available[i]))
	}
	return results, nil
}
