package service

import (
	"context"
	"strings"

	inventorypublic "github.com/nikhea/malawi-e-commerce-store/internal/inventory/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/variants/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/variants/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
)

var _ public.Service = (*service)(nil)

// Repository is the port this service needs from persistence.
type Repository interface {
	Create(ctx context.Context, v model.Variant) (model.Variant, error)
	GetByID(ctx context.Context, id string) (model.Variant, error)
	ListByProduct(ctx context.Context, productID string) ([]model.Variant, error)
	Update(ctx context.Context, v model.Variant) (model.Variant, error)
	Delete(ctx context.Context, id string) error
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

func (s *service) Create(ctx context.Context, in public.CreateVariantInput) (public.Variant, error) {
	if strings.TrimSpace(in.ProductID) == "" {
		return public.Variant{}, apperr.Validation("product id is required")
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return public.Variant{}, apperr.Validation("name is required")
	}
	sku := strings.ToUpper(strings.TrimSpace(in.SKU))
	if sku == "" {
		return public.Variant{}, apperr.Validation("sku is required")
	}
	if in.PriceCents < 0 {
		return public.Variant{}, apperr.Validation("price cannot be negative")
	}

	v, err := s.repo.Create(ctx, model.Variant{
		ProductID: in.ProductID, Name: name, SKU: sku, PriceCents: in.PriceCents,
	})
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

func (s *service) Update(ctx context.Context, id string, in public.UpdateVariantInput) (public.Variant, error) {
	if strings.TrimSpace(id) == "" {
		return public.Variant{}, apperr.Validation("id is required")
	}
	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return public.Variant{}, err
	}

	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return public.Variant{}, apperr.Validation("name is required")
		}
		current.Name = name
	}
	if in.PriceCents != nil {
		if *in.PriceCents < 0 {
			return public.Variant{}, apperr.Validation("price cannot be negative")
		}
		current.PriceCents = *in.PriceCents
	}
	if in.IsActive != nil {
		current.IsActive = *in.IsActive
	}
	// NOTE: product_id and sku are immutable (order history references
	// them); changing either means creating a new variant.

	// Read stock before the write so a stock lookup failure cannot leave
	// the caller with a 500 after the variant was already updated.
	stock, err := s.inventory.GetStock(ctx, current.ProductID, &current.ID)
	if err != nil {
		return public.Variant{}, err
	}
	updated, err := s.repo.Update(ctx, current)
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
