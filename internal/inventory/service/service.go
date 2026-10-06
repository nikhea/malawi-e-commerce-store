package service

import (
	"context"
	"strings"
	"time"

	"github.com/nikhea/malawi-e-commerce-store/internal/inventory/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/inventory/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
)

var _ public.Service = (*service)(nil)

// Repository is the port this service needs from persistence.
type Repository interface {
	UpsertStock(ctx context.Context, productID string, variantID *string, qty int) (model.Stock, error)
	GetStock(ctx context.Context, productID string, variantID *string) (model.Stock, error)
	Reserve(ctx context.Context, orderRef string, lines []model.Reservation, expiresAt time.Time) error
	ReleaseByOrder(ctx context.Context, orderRef string) error
	ConfirmByOrder(ctx context.Context, orderRef string) error
	ReleaseExpired(ctx context.Context) ([]string, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) public.Service {
	return &service{repo: repo}
}

func toPublic(s model.Stock) public.Stock {
	return public.Stock{
		ProductID: s.ProductID, VariantID: s.VariantID,
		OnHand: s.OnHand, Reserved: s.Reserved,
		Available: s.OnHand - s.Reserved,
	}
}

func (s *service) SetStock(ctx context.Context, productID string, variantID *string, qty int) (public.Stock, error) {
	if strings.TrimSpace(productID) == "" {
		return public.Stock{}, apperr.Validation("product id is required")
	}
	if qty < 0 {
		return public.Stock{}, apperr.Validation("qty cannot be negative")
	}
	stock, err := s.repo.UpsertStock(ctx, productID, variantID, qty)
	if err != nil {
		return public.Stock{}, err
	}
	return toPublic(stock), nil
}

func (s *service) GetStock(ctx context.Context, productID string, variantID *string) (public.Stock, error) {
	if strings.TrimSpace(productID) == "" {
		return public.Stock{}, apperr.Validation("product id is required")
	}
	stock, err := s.repo.GetStock(ctx, productID, variantID)
	if err != nil {
		return public.Stock{}, err
	}
	return toPublic(stock), nil
}

func (s *service) Reserve(ctx context.Context, orderRef string, lines []public.ReserveLine, ttl time.Duration) error {
	if strings.TrimSpace(orderRef) == "" {
		return apperr.Validation("order ref is required")
	}
	if len(lines) == 0 {
		return apperr.Validation("at least one line is required")
	}
	if ttl <= 0 {
		return apperr.Validation("ttl must be positive")
	}
	reservations := make([]model.Reservation, 0, len(lines))
	for _, l := range lines {
		if strings.TrimSpace(l.ProductID) == "" {
			return apperr.Validation("product id is required")
		}
		if l.Qty <= 0 {
			return apperr.Validation("qty must be positive")
		}
		reservations = append(reservations, model.Reservation{
			ProductID: l.ProductID, VariantID: l.VariantID, Qty: l.Qty,
		})
	}
	return s.repo.Reserve(ctx, orderRef, reservations, time.Now().Add(ttl))
}

func (s *service) ReleaseByOrder(ctx context.Context, orderRef string) error {
	if strings.TrimSpace(orderRef) == "" {
		return apperr.Validation("order ref is required")
	}
	return s.repo.ReleaseByOrder(ctx, orderRef)
}

func (s *service) ConfirmByOrder(ctx context.Context, orderRef string) error {
	if strings.TrimSpace(orderRef) == "" {
		return apperr.Validation("order ref is required")
	}
	return s.repo.ConfirmByOrder(ctx, orderRef)
}

func (s *service) ReleaseExpired(ctx context.Context) ([]string, error) {
	return s.repo.ReleaseExpired(ctx)
}
