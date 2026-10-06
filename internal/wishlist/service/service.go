package service

import (
	"context"
	"strings"

	productspublic "github.com/nikhea/malawi-e-commerce-store/internal/products/public"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/wishlist/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/wishlist/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
)

var _ public.Service = (*service)(nil)

// Repository is the port this service needs from persistence.
type Repository interface {
	Add(ctx context.Context, userID, productID string) (model.Item, error)
	ListByUser(ctx context.Context, userID string) ([]model.Item, error)
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

func (s *service) ListMine(ctx context.Context, userID string) ([]public.Item, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, apperr.Validation("user id is required")
	}
	items, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]public.Item, 0, len(items))
	for _, i := range items {
		attached, err := s.attach(ctx, i)
		if err != nil {
			// Product deleted between list and attach (FK cascade will
			// clean the link): skip instead of failing the whole list.
			continue
		}
		out = append(out, attached)
	}
	return out, nil
}
