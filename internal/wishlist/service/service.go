package service

import (
	"context"
	"strings"

	productspublic "github.com/nikhea/malawi-e-commerce-store/internal/products/public"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/wishlist/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/wishlist/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	"github.com/nikhea/malawi-e-commerce-store/pkg/pagination"
)

var _ public.Service = (*service)(nil)

// Repository is the port this service needs from persistence.
type Repository interface {
	Add(ctx context.Context, userID, productID string) (model.Item, error)
	ListByUser(ctx context.Context, userID string, f model.ListFilter) ([]model.Item, int64, error)
	Remove(ctx context.Context, userID, productID string) error
}

type service struct {
	repo     Repository
	products productspublic.Service
	users    userspublic.Service
}

func NewService(repo Repository, products productspublic.Service, users userspublic.Service) public.Service {
	return &service{repo: repo, products: products, users: users}
}

func (s *service) attach(ctx context.Context, i model.Item) (public.Item, error) {
	detail, err := s.products.GetByID(ctx, i.ProductID)
	if err != nil {
		return public.Item{}, err
	}
	p := detail.Product
	return public.Item{
		ProductID: i.ProductID,
		Product: public.ProductSummary{
			ID: p.ID, Name: p.Name, Slug: p.Slug,
			PriceCents: p.PriceCents, Currency: p.Currency, ImageURL: p.ImageURL,
		},
		AddedAt: i.CreatedAt,
	}, nil
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
	item, err := s.repo.Add(ctx, userID, productID)
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
