package service

import (
	"context"
	"strings"
	"time"

	cartpublic "github.com/nikhea/malawi-e-commerce-store/internal/cart/public"
	inventorypublic "github.com/nikhea/malawi-e-commerce-store/internal/inventory/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/orders/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/orders/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	"github.com/nikhea/malawi-e-commerce-store/pkg/events"
)

var _ public.Service = (*service)(nil)

// Repository is the port this service needs from persistence.
type Repository interface {
	Create(ctx context.Context, o model.Order, items []model.Item) (model.Order, error)
	GetByID(ctx context.Context, orderID string) (model.Order, error)
	ListByUser(ctx context.Context, userID string) ([]model.Order, error)
	MarkPaid(ctx context.Context, orderID string) (model.Order, error)
	Cancel(ctx context.Context, orderID string) (model.Order, error)
	GetItems(ctx context.Context, orderID string) ([]model.Item, error)
}

// Config carries checkout tunables.
type Config struct {
	// ReserveTTL holds stock per checkout (abandoned carts release).
	ReserveTTL time.Duration
}

type service struct {
	repo      Repository
	cart      cartpublic.Service
	inventory inventorypublic.Service
	bus       *events.Bus
	ttl       time.Duration
}

func NewService(repo Repository, cart cartpublic.Service, inventory inventorypublic.Service, bus *events.Bus, cfg Config) public.Service {
	ttl := cfg.ReserveTTL
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	return &service{repo: repo, cart: cart, inventory: inventory, bus: bus, ttl: ttl}
}

func toPublic(o model.Order, items []model.Item) public.Order {
	lines := make([]public.OrderLine, 0, len(items))
	for _, i := range items {
		lines = append(lines, public.OrderLine{
			ProductID: i.ProductID, VariantID: i.VariantID, Name: i.Name,
			SKU: i.SKU, UnitPriceCents: i.UnitPriceCents, Qty: i.Qty,
			LineTotalCents: i.UnitPriceCents * int64(i.Qty),
		})
	}
	return public.Order{
		ID: o.ID, UserID: o.UserID, Status: o.Status,
		SubtotalCents: o.SubtotalCents, Currency: o.Currency,
		Lines: lines, CreatedAt: o.CreatedAt,
	}
}

func (s *service) view(ctx context.Context, o model.Order) (public.Order, error) {
	items, err := s.repo.GetItems(ctx, o.ID)
	if err != nil {
		return public.Order{}, err
	}
	return toPublic(o, items), nil
}

func (s *service) Checkout(ctx context.Context, userID string) (public.Order, error) {
	if strings.TrimSpace(userID) == "" {
		return public.Order{}, apperr.Validation("user id is required")
	}
	cart, err := s.cart.Get(ctx, userID)
	if err != nil {
		return public.Order{}, err
	}
	if len(cart.Items) == 0 {
		return public.Order{}, apperr.Validation("cart is empty")
	}

	items := make([]model.Item, 0, len(cart.Items))
	lines := make([]inventorypublic.ReserveLine, 0, len(cart.Items))
	for _, l := range cart.Items {
		items = append(items, model.Item{
			ProductID: l.ProductID, VariantID: l.VariantID, Name: l.Name,
			SKU: l.SKU, UnitPriceCents: l.UnitPriceCents, Qty: l.Qty,
		})
		lines = append(lines, inventorypublic.ReserveLine{
			ProductID: l.ProductID, VariantID: l.VariantID, Qty: l.Qty,
		})
	}
	order, err := s.repo.Create(ctx, model.Order{
		UserID: userID, Status: public.StatusPending,
		SubtotalCents: cart.SubtotalCents, Currency: cart.Currency,
	}, items)
	if err != nil {
		return public.Order{}, err
	}

	// Reserve AFTER the order exists (the ref must exist first). On any
	// failure below, compensate so no orphan pending order or held stock
	// survives a failed checkout.
	if err := s.inventory.Reserve(ctx, order.ID, lines, s.ttl); err != nil {
		_, _ = s.repo.Cancel(ctx, order.ID)
		return public.Order{}, err
	}
	if err := s.cart.Clear(ctx, userID); err != nil {
		_ = s.inventory.ReleaseByOrder(ctx, order.ID)
		_, _ = s.repo.Cancel(ctx, order.ID)
		return public.Order{}, apperr.Internal(err)
	}

	view, err := s.view(ctx, order)
	if err != nil {
		return public.Order{}, err
	}
	s.bus.Publish(ctx, events.Event{Name: public.OrderCreated, Payload: view})
	return view, nil
}

func (s *service) owned(ctx context.Context, userID, orderID string) (model.Order, error) {
	if strings.TrimSpace(orderID) == "" {
		return model.Order{}, apperr.Validation("order id is required")
	}
	o, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return model.Order{}, err
	}
	if o.UserID != userID {
		// Same code as unknown id: don't leak other users' orders.
		return model.Order{}, apperr.NotFound("order not found")
	}
	return o, nil
}

func (s *service) GetByID(ctx context.Context, userID, orderID string) (public.Order, error) {
	o, err := s.owned(ctx, userID, orderID)
	if err != nil {
		return public.Order{}, err
	}
	return s.view(ctx, o)
}

func (s *service) ListMine(ctx context.Context, userID string) ([]public.Order, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, apperr.Validation("user id is required")
	}
	orders, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]public.Order, 0, len(orders))
	for _, o := range orders {
		v, err := s.view(ctx, o)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (s *service) Cancel(ctx context.Context, userID, orderID string) (public.Order, error) {
	o, err := s.owned(ctx, userID, orderID)
	if err != nil {
		return public.Order{}, err
	}
	return s.cancelOrder(ctx, o)
}

func (s *service) cancelOrder(ctx context.Context, o model.Order) (public.Order, error) {
	if o.Status == public.StatusCancelled {
		return s.view(ctx, o) // idempotent
	}
	if o.Status != public.StatusPending {
		return public.Order{}, apperr.Conflict("order is not pending")
	}
	if err := s.inventory.ReleaseByOrder(ctx, o.ID); err != nil {
		return public.Order{}, err
	}
	cancelled, err := s.repo.Cancel(ctx, o.ID)
	if err != nil {
		return public.Order{}, err
	}
	view, err := s.view(ctx, cancelled)
	if err != nil {
		return public.Order{}, err
	}
	s.bus.Publish(ctx, events.Event{Name: public.OrderCancelled, Payload: view})
	return view, nil
}

// MarkPaid is crash-safe in both directions: the order row is the source
// of truth, the idempotent confirm closes any crash window, and repeats
// return the paid order (webhook retries must succeed).
func (s *service) MarkPaid(ctx context.Context, orderRef string) (public.Order, error) {
	if strings.TrimSpace(orderRef) == "" {
		return public.Order{}, apperr.Validation("order ref is required")
	}
	o, err := s.repo.GetByID(ctx, orderRef)
	if err != nil {
		return public.Order{}, err
	}
	switch o.Status {
	case public.StatusPaid:
		return s.view(ctx, o) // idempotent repeat
	case public.StatusCancelled:
		return public.Order{}, apperr.Conflict("order is cancelled")
	case public.StatusPending:
		// fall through
	default:
		return public.Order{}, apperr.Conflict("order is not pending")
	}

	paid, err := s.repo.MarkPaid(ctx, o.ID)
	if err != nil {
		return public.Order{}, err
	}
	if err := s.inventory.ConfirmByOrder(ctx, o.ID); err != nil {
		return public.Order{}, err
	}
	view, err := s.view(ctx, paid)
	if err != nil {
		return public.Order{}, err
	}
	s.bus.Publish(ctx, events.Event{Name: public.OrderPaid, Payload: view})
	return view, nil
}

func (s *service) CancelByRef(ctx context.Context, orderRef string) error {
	if strings.TrimSpace(orderRef) == "" {
		return apperr.Validation("order ref is required")
	}
	o, err := s.repo.GetByID(ctx, orderRef)
	if err != nil {
		return err
	}
	_, err = s.cancelOrder(ctx, o)
	return err
}
