package service

import (
	"context"
	"strings"
	"time"

	"github.com/nikhea/malawi-e-commerce-store/internal/inventory/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/inventory/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	"github.com/nikhea/malawi-e-commerce-store/pkg/bulk"
)

var _ public.Service = (*service)(nil)

// Repository is the port this service needs from persistence.
type Repository interface {
	UpsertStock(ctx context.Context, productID string, variantID *string, qty int) (model.Stock, error)
	BulkUpsertStock(ctx context.Context, items []model.StockInput, atomic bool) ([]model.Stock, []error, error)
	GetStock(ctx context.Context, productID string, variantID *string) (model.Stock, error)
	ListStockByProduct(ctx context.Context, productID string) ([]model.Stock, error)
	ListStockByVariantIDs(ctx context.Context, variantIDs []string) ([]model.Stock, error)
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

func stockID(it public.BulkStockItem) string {
	if it.VariantID != nil {
		return *it.VariantID
	}
	return it.ProductID
}

func validateStockItem(it public.BulkStockItem) error {
	if strings.TrimSpace(it.ProductID) == "" {
		return apperr.Validation("product id is required")
	}
	if it.Qty < 0 {
		return apperr.Validation("qty cannot be negative")
	}
	return nil
}

// BulkSetStock sets many on-hand quantities. Non-atomic writes run one by
// one; atomic writes validate everything first and then share one
// transaction, so a single bad line leaves every row untouched.
func (s *service) BulkSetStock(ctx context.Context, items []public.BulkStockItem, atomic bool) ([]bulk.BulkResult[public.Stock], error) {
	if err := bulk.CheckCount(len(items), bulk.MaxItems()); err != nil {
		return nil, err
	}
	keys := make([]string, len(items))
	for i, it := range items {
		keys[i] = bulk.LineKey(it.ProductID, it.VariantID)
	}
	if dup, ok := bulk.FindDuplicate(keys); ok {
		return nil, apperr.Duplicate("duplicate stock line: " + dup)
	}

	results := make([]bulk.BulkResult[public.Stock], len(items))
	for i, it := range items {
		results[i] = bulk.BulkResult[public.Stock]{Index: i, ID: stockID(it), Status: "ok"}
	}

	if !atomic {
		for i, it := range items {
			stock, err := s.SetStock(ctx, it.ProductID, it.VariantID, it.Qty)
			if err != nil {
				results[i] = bulk.Failed[public.Stock](i, stockID(it), err)
				continue
			}
			results[i] = bulk.OK(i, stockID(it), stock)
		}
		return results, nil
	}

	for i, it := range items {
		if err := validateStockItem(it); err != nil {
			return bulk.Rollback(results, i, err), nil
		}
	}
	inputs := make([]model.StockInput, len(items))
	for i, it := range items {
		inputs[i] = model.StockInput{ProductID: it.ProductID, VariantID: it.VariantID, Qty: it.Qty}
	}
	stocks, errs, err := s.repo.BulkUpsertStock(ctx, inputs, true)
	if err != nil {
		return bulk.Rollback(results, 0, err), nil
	}
	for i, itemErr := range errs {
		if itemErr != nil {
			return bulk.Rollback(results, i, itemErr), nil
		}
	}
	for i := range items {
		results[i] = bulk.OK(i, stockID(items[i]), toPublic(stocks[i]))
	}
	return results, nil
}

// GetStockByVariants reads availability for many variants in one query and
// reports ids that do not resolve to a variant.
func (s *service) GetStockByVariants(ctx context.Context, variantIDs []string) ([]public.Stock, []string, error) {
	if err := bulk.CheckCount(len(variantIDs), bulk.MaxItems()); err != nil {
		return nil, nil, err
	}
	if dup, ok := bulk.FindDuplicate(variantIDs); ok {
		return nil, nil, apperr.Duplicate("duplicate variant id: " + dup)
	}
	rows, err := s.repo.ListStockByVariantIDs(ctx, variantIDs)
	if err != nil {
		return nil, nil, err
	}
	byID := make(map[string]public.Stock, len(rows))
	for _, row := range rows {
		if row.VariantID != nil {
			byID[*row.VariantID] = toPublic(row)
		}
	}
	out := make([]public.Stock, 0, len(rows))
	notFound := []string{}
	for _, id := range variantIDs {
		stock, ok := byID[id]
		if !ok {
			notFound = append(notFound, id)
			continue
		}
		out = append(out, stock)
	}
	return out, notFound, nil
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

func (s *service) ListStockByProduct(ctx context.Context, productID string) ([]public.Stock, error) {
	if strings.TrimSpace(productID) == "" {
		return nil, apperr.Validation("product id is required")
	}
	rows, err := s.repo.ListStockByProduct(ctx, productID)
	if err != nil {
		return nil, err
	}
	out := make([]public.Stock, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPublic(row))
	}
	return out, nil
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
