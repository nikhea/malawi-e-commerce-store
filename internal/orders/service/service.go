package service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	cartpublic "github.com/nikhea/malawi-e-commerce-store/internal/cart/public"
	inventorypublic "github.com/nikhea/malawi-e-commerce-store/internal/inventory/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/orders/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/orders/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	"github.com/nikhea/malawi-e-commerce-store/pkg/pagination"
)

var _ public.Service = (*service)(nil)

// Repository is the port this service needs from persistence.
type Repository interface {
	Create(ctx context.Context, o model.Order, items []model.Item) (model.Order, error)
	GetByID(ctx context.Context, orderID string) (model.Order, error)
	ListByUser(ctx context.Context, userID string, f model.ListFilter) ([]model.Order, int64, error)
	Aggregate(ctx context.Context, userID string, f model.AggregateFilter) (model.Aggregates, error)
	MarkPaid(ctx context.Context, orderID string) (model.Order, error)
	Cancel(ctx context.Context, orderID string) (model.Order, error)
	GetItems(ctx context.Context, orderID string) ([]model.Item, error)
}

// Config carries checkout tunables.
type Config struct {
	// ReserveTTL holds stock per checkout (abandoned carts release).
	ReserveTTL time.Duration
}

// Notifier is the order-lifecycle fan-out port. Production:
// *notify.Service (durable River mail + logs). Tests: a recording fake.
type Notifier interface {
	OrderCreated(ctx context.Context, o public.Order)
	OrderPaid(ctx context.Context, o public.Order)
	OrderCancelled(ctx context.Context, o public.Order)
}

type service struct {
	repo      Repository
	cart      cartpublic.Service
	inventory inventorypublic.Service
	notify    Notifier
	ttl       time.Duration
}

func NewService(repo Repository, cart cartpublic.Service, inventory inventorypublic.Service, notify Notifier, cfg Config) public.Service {
	ttl := cfg.ReserveTTL
	if ttl <= 0 {
		ttl = public.ReserveTTL
	}
	return &service{repo: repo, cart: cart, inventory: inventory, notify: notify, ttl: ttl}
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
	s.notify.OrderCreated(ctx, view)
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

func (s *service) GetByRef(ctx context.Context, orderID string) (public.Order, error) {
	if strings.TrimSpace(orderID) == "" {
		return public.Order{}, apperr.Validation("order id is required")
	}
	o, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return public.Order{}, err
	}
	return s.view(ctx, o)
}

var orderSorts = map[string]bool{
	"newest":     true,
	"oldest":     true,
	"total_asc":  true,
	"total_desc": true,
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
	status := strings.ToLower(strings.TrimSpace(f.Status))
	switch status {
	case "", public.StatusPending, public.StatusPaid, public.StatusCancelled:
	default:
		return public.Page{}, apperr.Validation("status must be pending, paid or cancelled")
	}
	if f.MinTotalCents != nil && f.MaxTotalCents != nil && *f.MinTotalCents > *f.MaxTotalCents {
		return public.Page{}, apperr.Validation("min_total_cents cannot exceed max_total_cents")
	}
	if f.CreatedAfter != nil && f.CreatedBefore != nil && f.CreatedAfter.After(*f.CreatedBefore) {
		return public.Page{}, apperr.Validation("created_after cannot be after created_before")
	}
	sortKey := strings.TrimSpace(f.Sort)
	if sortKey == "" {
		sortKey = "newest"
	}
	if !orderSorts[sortKey] {
		return public.Page{}, apperr.Validation("unknown sort; use newest, oldest, total_asc or total_desc")
	}

	limit, offset := pagination.NormalizeWithDefault(f.Page, f.Limit, f.Offset, pagination.DefaultListLimit)
	orders, total, err := s.repo.ListByUser(ctx, userID, model.ListFilter{
		Status: status, Currency: strings.TrimSpace(f.Currency),
		CreatedAfter: f.CreatedAfter, CreatedBefore: f.CreatedBefore,
		MinTotalCents: f.MinTotalCents, MaxTotalCents: f.MaxTotalCents,
		Sort: sortKey, Limit: limit, Offset: offset,
	})
	if err != nil {
		return public.Page{}, err
	}
	out := make([]public.Order, 0, len(orders))
	for _, o := range orders {
		v, err := s.view(ctx, o)
		if err != nil {
			return public.Page{}, err
		}
		out = append(out, v)
	}
	return public.Page{Items: out, Meta: pagination.NewMeta(total, limit, offset)}, nil
}

// aggregateModel converts the repository's module-private totals into the
// public wire shape. Slices are non-nil so an empty result serializes as []
// rather than null.
func aggregateModel(in model.Aggregates) public.Aggregates {
	out := public.Aggregates{
		Orders: public.OrdersAggregate{
			Count:        in.Orders.Count,
			FirstOrderAt: in.Orders.FirstOrderAt,
			LastOrderAt:  in.Orders.LastOrderAt,
			ByStatus:     make([]public.StatusTotal, 0, len(in.Orders.ByStatus)),
			ByCurrency:   make([]public.CurrencyTotal, 0, len(in.Orders.ByCurrency)),
		},
		Items: public.ItemsAggregate{
			Lines:             in.Items.Lines,
			UnitsSold:         in.Items.UnitsSold,
			RevenueByCurrency: make([]public.CurrencyRevenue, 0, len(in.Items.RevenueByCurrency)),
			TopProducts:       make([]public.ProductTotal, 0, len(in.Items.TopProducts)),
		},
	}
	for _, st := range in.Orders.ByStatus {
		out.Orders.ByStatus = append(out.Orders.ByStatus, public.StatusTotal{
			Status: st.Status, Currency: st.Currency, Orders: st.Orders,
			TotalCents: st.TotalCents, AverageCents: st.AverageCents,
		})
	}
	for _, ct := range in.Orders.ByCurrency {
		out.Orders.ByCurrency = append(out.Orders.ByCurrency, public.CurrencyTotal{
			Currency: ct.Currency, Orders: ct.Orders,
			TotalCents: ct.TotalCents, AverageCents: ct.AverageCents,
		})
	}
	for _, cr := range in.Items.RevenueByCurrency {
		out.Items.RevenueByCurrency = append(out.Items.RevenueByCurrency, public.CurrencyRevenue{
			Currency: cr.Currency, Lines: cr.Lines,
			UnitsSold: cr.UnitsSold, RevenueCents: cr.RevenueCents,
		})
	}
	for _, p := range in.Items.TopProducts {
		out.Items.TopProducts = append(out.Items.TopProducts, public.ProductTotal{
			ProductID: p.ProductID, Name: p.Name, SKU: p.SKU, Currency: p.Currency,
			UnitsSold: p.UnitsSold, Orders: p.Orders, RevenueCents: p.RevenueCents,
		})
	}
	return out
}

// aggregateFilter validates the shared query and normalizes it for the
// repository. Status/currency casing is handled here, not in the handler.
func aggregateFilter(f public.AggregateFilter) (model.AggregateFilter, error) {
	status := strings.ToLower(strings.TrimSpace(f.Status))
	switch status {
	case "", public.StatusPending, public.StatusPaid, public.StatusCancelled:
	default:
		return model.AggregateFilter{}, apperr.Validation("status must be pending, paid or cancelled")
	}
	if f.CreatedAfter != nil && f.CreatedBefore != nil && f.CreatedAfter.After(*f.CreatedBefore) {
		return model.AggregateFilter{}, apperr.Validation("created_after cannot be after created_before")
	}
	return model.AggregateFilter{
		Status: status, Currency: strings.TrimSpace(f.Currency),
		CreatedAfter: f.CreatedAfter, CreatedBefore: f.CreatedBefore,
		TopProducts: public.TopProductsLimit,
	}, nil
}

func (s *service) aggregate(ctx context.Context, userID string, f public.AggregateFilter) (public.Aggregates, error) {
	mf, err := aggregateFilter(f)
	if err != nil {
		return public.Aggregates{}, err
	}
	in, err := s.repo.Aggregate(ctx, userID, mf)
	if err != nil {
		return public.Aggregates{}, err
	}
	return aggregateModel(in), nil
}

func (s *service) AggregateMine(ctx context.Context, userID string, f public.AggregateFilter) (public.Aggregates, error) {
	if strings.TrimSpace(userID) == "" {
		return public.Aggregates{}, apperr.Validation("user id is required")
	}
	return s.aggregate(ctx, userID, f)
}

func (s *service) AggregateAll(ctx context.Context, userID string, f public.AggregateFilter) (public.Aggregates, error) {
	userID = strings.TrimSpace(userID)
	if userID != "" {
		// This value comes from a query string, not a token: reject garbage
		// before handing it to a UUID column (which would surface as a 500).
		if _, err := uuid.Parse(userID); err != nil {
			return public.Aggregates{}, apperr.Validation("user_id must be a UUID")
		}
	}
	return s.aggregate(ctx, userID, f)
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
		// Idempotent, but still release: an earlier attempt may have won
		// the transition and crashed before freeing the hold.
		if err := s.inventory.ReleaseByOrder(ctx, o.ID); err != nil {
			return public.Order{}, err
		}
		return s.view(ctx, o)
	}
	if o.Status != public.StatusPending {
		return public.Order{}, apperr.Conflict("order is not pending")
	}
	// Claim the transition BEFORE releasing stock. If a concurrent MarkPaid
	// already won pending → paid, Cancel fails and we must not free a hold
	// for a paid order.
	cancelled, err := s.repo.Cancel(ctx, o.ID)
	if err != nil {
		return public.Order{}, err
	}
	if err := s.inventory.ReleaseByOrder(ctx, o.ID); err != nil {
		// The order is cancelled; a retry takes the idempotent branch above
		// and releases the hold then.
		return public.Order{}, err
	}
	view, err := s.view(ctx, cancelled)
	if err != nil {
		return public.Order{}, err
	}
	s.notify.OrderCancelled(ctx, view)
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
		// Idempotent repeat — but re-run the confirm: a previous attempt
		// could have flipped the order and crashed before settling stock.
		// ConfirmByOrder is a no-op when there are no active holds.
		if err := s.inventory.ConfirmByOrder(ctx, o.ID); err != nil {
			return public.Order{}, err
		}
		return s.view(ctx, o)
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
	s.notify.OrderPaid(ctx, view)
	return view, nil
}

// ExtendCheckoutHold refreshes the hold for a pending order. Payments calls
// it when a hosted session is created so the hold cannot expire mid-payment.
func (s *service) ExtendCheckoutHold(ctx context.Context, orderID string) (time.Time, error) {
	if strings.TrimSpace(orderID) == "" {
		return time.Time{}, apperr.Validation("order id is required")
	}
	o, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return time.Time{}, err
	}
	if o.Status != public.StatusPending {
		return time.Time{}, apperr.Conflict("order is not pending")
	}
	return s.inventory.ExtendReservation(ctx, o.ID, s.ttl)
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

// SettleExpired resolves one expired stock hold by the order's real status:
// a pending order is cancelled (transition first, then release), a paid
// order confirms the hold (the payment won the race with the sweep), and an
// already cancelled order just releases. Safe to repeat, so a sweep that
// fails halfway converges on the next run.
func (s *service) SettleExpired(ctx context.Context, orderRef string) error {
	if strings.TrimSpace(orderRef) == "" {
		return apperr.Validation("order ref is required")
	}
	o, err := s.repo.GetByID(ctx, orderRef)
	if err != nil {
		if apperr.CodeOf(err) == apperr.CodeNotFound {
			return nil // order deleted: nothing left to settle
		}
		return err
	}
	switch o.Status {
	case public.StatusPending:
		_, err = s.cancelOrder(ctx, o)
		return err
	case public.StatusPaid:
		return s.inventory.ConfirmByOrder(ctx, o.ID)
	case public.StatusCancelled:
		return s.inventory.ReleaseByOrder(ctx, o.ID)
	default:
		return nil
	}
}
