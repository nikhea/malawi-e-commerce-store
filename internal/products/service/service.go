package service

import (
	"context"
	"strings"

	categoriespublic "github.com/nikhea/malawi-e-commerce-store/internal/categories/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/products/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/products/public"
	variantspublic "github.com/nikhea/malawi-e-commerce-store/internal/variants/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	"github.com/nikhea/malawi-e-commerce-store/pkg/slug"
)

var _ public.Service = (*service)(nil)

// Repository is the port this service needs from persistence.
type Repository interface {
	Create(ctx context.Context, p model.Product) (model.Product, error)
	GetByID(ctx context.Context, id string) (model.Product, error)
	GetBySlug(ctx context.Context, slug string) (model.Product, error)
	List(ctx context.Context, categoryID *string, activeOnly bool, limit, offset int) ([]model.Product, error)
	Update(ctx context.Context, p model.Product) (model.Product, error)
	Delete(ctx context.Context, id string) error
	SetImage(ctx context.Context, id, url, publicID string) (model.Product, error)
}

type service struct {
	repo       Repository
	categories categoriespublic.Service
	variants   variantspublic.Service
}

func NewService(repo Repository, categories categoriespublic.Service, variants variantspublic.Service) public.Service {
	return &service{repo: repo, categories: categories, variants: variants}
}

func toPublic(p model.Product) public.Product {
	return public.Product{
		ID: p.ID, CategoryID: p.CategoryID, CategoryName: p.CategoryName,
		Name: p.Name, Slug: p.Slug, Description: p.Description,
		PriceCents: p.PriceCents, Currency: p.Currency,
		ImageURL: p.ImageURL, IsActive: p.IsActive, CreatedAt: p.CreatedAt,
	}
}

func toDetail(p model.Product, variants []variantspublic.Variant) public.Detail {
	views := make([]public.VariantView, 0, len(variants))
	for _, v := range variants {
		views = append(views, public.VariantView{
			ID: v.ID, Name: v.Name, SKU: v.SKU,
			PriceCents: v.PriceCents, IsActive: v.IsActive,
		})
	}
	return public.Detail{Product: toPublic(p), Variants: views}
}

func (s *service) detail(ctx context.Context, p model.Product) (public.Detail, error) {
	variants, err := s.variants.ListByProduct(ctx, p.ID)
	if err != nil {
		return public.Detail{}, err
	}
	return toDetail(p, variants), nil
}

func (s *service) Create(ctx context.Context, in public.CreateProductInput) (public.Product, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return public.Product{}, apperr.Validation("name is required")
	}
	slugStr := strings.ToLower(strings.TrimSpace(in.Slug))
	if slugStr == "" {
		slugStr = slug.Make(name)
	}
	if slugStr == "" {
		return public.Product{}, apperr.Validation("slug is required")
	}
	if in.PriceCents < 0 {
		return public.Product{}, apperr.Validation("price cannot be negative")
	}
	currency := strings.ToUpper(strings.TrimSpace(in.Currency))
	if currency == "" {
		currency = "MWK"
	}
	if in.CategoryID != nil {
		if _, err := s.categories.GetByID(ctx, *in.CategoryID); err != nil {
			return public.Product{}, err // NOT_FOUND passes through
		}
	}

	p, err := s.repo.Create(ctx, model.Product{
		CategoryID: in.CategoryID, Name: name, Slug: slugStr,
		Description: strings.TrimSpace(in.Description),
		PriceCents:  in.PriceCents, Currency: currency,
		ImageURL: strings.TrimSpace(in.ImageURL),
	})
	if err != nil {
		return public.Product{}, err
	}
	return toPublic(p), nil
}

func (s *service) GetByID(ctx context.Context, id string) (public.Detail, error) {
	if strings.TrimSpace(id) == "" {
		return public.Detail{}, apperr.Validation("id is required")
	}
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return public.Detail{}, err
	}
	return s.detail(ctx, p)
}

func (s *service) GetBySlug(ctx context.Context, slugStr string) (public.Detail, error) {
	slugStr = strings.ToLower(strings.TrimSpace(slugStr))
	if slugStr == "" {
		return public.Detail{}, apperr.Validation("slug is required")
	}
	p, err := s.repo.GetBySlug(ctx, slugStr)
	if err != nil {
		return public.Detail{}, err
	}
	return s.detail(ctx, p)
}

func (s *service) List(ctx context.Context, f public.ListFilter) ([]public.Product, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if f.Offset < 0 {
		return nil, apperr.Validation("offset cannot be negative")
	}
	products, err := s.repo.List(ctx, f.CategoryID, f.ActiveOnly, limit, f.Offset)
	if err != nil {
		return nil, err
	}
	out := make([]public.Product, 0, len(products))
	for _, p := range products {
		out = append(out, toPublic(p))
	}
	return out, nil
}

func (s *service) Update(ctx context.Context, id string, in public.UpdateProductInput) (public.Product, error) {
	if strings.TrimSpace(id) == "" {
		return public.Product{}, apperr.Validation("id is required")
	}
	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return public.Product{}, err
	}

	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return public.Product{}, apperr.Validation("name is required")
		}
		current.Name = name
	}
	if in.Description != nil {
		current.Description = strings.TrimSpace(*in.Description)
	}
	if in.PriceCents != nil {
		if *in.PriceCents < 0 {
			return public.Product{}, apperr.Validation("price cannot be negative")
		}
		current.PriceCents = *in.PriceCents
	}
	if in.Currency != nil {
		currency := strings.ToUpper(strings.TrimSpace(*in.Currency))
		if currency == "" {
			return public.Product{}, apperr.Validation("currency is required")
		}
		current.Currency = currency
	}
	if in.IsActive != nil {
		current.IsActive = *in.IsActive
	}
	switch {
	case in.ClearCategory:
		current.CategoryID = nil
	case in.CategoryID != nil:
		if _, err := s.categories.GetByID(ctx, *in.CategoryID); err != nil {
			return public.Product{}, err
		}
		current.CategoryID = in.CategoryID
	}
	// NOTE: slug is immutable (URLs must not rot).

	updated, err := s.repo.Update(ctx, current)
	if err != nil {
		return public.Product{}, err
	}
	return toPublic(updated), nil
}

func (s *service) Delete(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return apperr.Validation("id is required")
	}
	return s.repo.Delete(ctx, id)
}

func (s *service) SetImage(ctx context.Context, id, url, publicID string) (public.Product, error) {
	if strings.TrimSpace(id) == "" {
		return public.Product{}, apperr.Validation("id is required")
	}
	if strings.TrimSpace(url) == "" {
		return public.Product{}, apperr.Validation("image url is required")
	}
	p, err := s.repo.SetImage(ctx, id, strings.TrimSpace(url), strings.TrimSpace(publicID))
	if err != nil {
		return public.Product{}, err
	}
	return toPublic(p), nil
}
