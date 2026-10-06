package service

import (
	"context"
	"math"
	"strings"

	orderspublic "github.com/nikhea/malawi-e-commerce-store/internal/orders/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/payments/gateway"
	"github.com/nikhea/malawi-e-commerce-store/internal/payments/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/payments/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
)

var _ public.Service = (*service)(nil)

// stripeMinCents is Stripe's minimum charge ($0.50). Converted totals
// below this are rejected instead of failing obscurely at Stripe.
const stripeMinCents = 50

// chargeCurrency is what Stripe bills. MWK is unsupported, so MWK
// tambala convert at Config.FXRate (a real FX provider plugs in here).
const chargeCurrency = "usd"

// Repository is the port this service needs from persistence.
type Repository interface {
	Create(ctx context.Context, p model.Payment) (model.Payment, error)
	GetByOrder(ctx context.Context, orderID string) (model.Payment, error)
	GetByIntent(ctx context.Context, intentID string) (model.Payment, error)
	MarkStatus(ctx context.Context, intentID, status string) (model.Payment, error)
}

// Config carries payments tunables.
type Config struct {
	WebhookSecret string
	FXMWKPerUSD   float64
}

type service struct {
	repo   Repository
	orders orderspublic.Service
	gw     gateway.Gateway
	secret string
	fxRate float64
	notify Notifier
}

// Notifier is the payment-outcome fan-out port. Production:
// *notify.Service (durable River mail + logs). Tests: a recording fake.
type Notifier interface {
	PaymentSucceeded(ctx context.Context, p public.Payment)
	PaymentFailed(ctx context.Context, p public.Payment)
}

func NewService(repo Repository, orders orderspublic.Service, gw gateway.Gateway, notify Notifier, cfg Config) public.Service {
	rate := cfg.FXMWKPerUSD
	if rate <= 0 {
		rate = 1700
	}
	return &service{repo: repo, orders: orders, gw: gw, secret: cfg.WebhookSecret, fxRate: rate, notify: notify}
}

// toStripeCents converts MWK tambala to billable USD cents.
func toStripeCents(mwkTambala int64, rate float64) int64 {
	return int64(math.Round(float64(mwkTambala) / rate))
}

func toPublic(p model.Payment) public.Payment {
	return public.Payment{
		ID: p.ID, OrderID: p.OrderID, StripeIntentID: p.StripeIntentID,
		ClientSecret: p.ClientSecret, AmountCents: p.AmountCents,
		Currency: p.Currency, OrderAmountCents: p.OrderAmountCents,
		Status: p.Status, CreatedAt: p.CreatedAt,
	}
}

func (s *service) CreateIntent(ctx context.Context, userID, orderID string) (public.Payment, error) {
	if strings.TrimSpace(orderID) == "" {
		return public.Payment{}, apperr.Validation("order id is required")
	}
	order, err := s.orders.GetByID(ctx, userID, orderID)
	if err != nil {
		return public.Payment{}, err // NOT_FOUND also covers foreign orders
	}
	if order.Status != orderspublic.StatusPending {
		return public.Payment{}, apperr.Conflict("order is not pending")
	}

	usd := toStripeCents(order.SubtotalCents, s.fxRate)
	if usd < stripeMinCents {
		return public.Payment{}, apperr.Validation("order total is below the minimum charge")
	}

	// Retry-safe: an existing row reuses its intent (no double charge).
	if existing, err := s.repo.GetByOrder(ctx, orderID); err == nil {
		if existing.Status == public.StatusSucceeded {
			return public.Payment{}, apperr.Conflict("order already paid")
		}
		return toPublic(existing), nil
	}

	intent, err := s.gw.CreateIntent(ctx, usd, chargeCurrency, orderID)
	if err != nil {
		return public.Payment{}, apperr.Internal(err)
	}
	p, err := s.repo.Create(ctx, model.Payment{
		OrderID: orderID, StripeIntentID: intent.ID, ClientSecret: intent.ClientSecret,
		AmountCents: usd, Currency: "USD",
		OrderAmountCents: order.SubtotalCents, Status: public.StatusPending,
	})
	if err != nil {
		return public.Payment{}, err
	}
	return toPublic(p), nil
}

func (s *service) HandleWebhook(ctx context.Context, payload []byte, sigHeader string) error {
	if len(payload) == 0 || strings.TrimSpace(sigHeader) == "" {
		return apperr.Validation("invalid webhook")
	}
	evt, err := s.gw.VerifyWebhook(payload, sigHeader, s.secret)
	if err != nil {
		// Forgery or corruption: 4xx so Stripe does NOT retry.
		return apperr.Validation("invalid signature")
	}

	switch evt.Type {
	case "payment_intent.succeeded":
		return s.settle(ctx, evt, public.StatusSucceeded,
			func(ctx context.Context, ref string) error {
				_, err := s.orders.MarkPaid(ctx, ref)
				return err
			})
	case "payment_intent.payment_failed", "payment_intent.canceled":
		return s.settle(ctx, evt, public.StatusFailed,
			s.orders.CancelByRef)
	default:
		// Unknown kinds (refunds, disputes, future types): acknowledge,
		// handle when a subscriber needs them. 200, no-op.
		return nil
	}
}

// settle moves the ledger row and the order together. The order call is
// idempotent, so Stripe retries converge instead of duplicating.
func (s *service) settle(ctx context.Context, evt gateway.WebhookEvent, status string, move func(context.Context, string) error) error {
	payment, err := s.repo.GetByIntent(ctx, evt.IntentID)
	if err != nil {
		if apperr.CodeOf(err) == apperr.CodeNotFound {
			// Unknown intent (another account's event, or a Stripe CLI
			// test trigger): acknowledge without touching orders.
			return nil
		}
		return err // DB failure: 5xx so Stripe retries.
	}
	if payment.Status == status {
		return nil // already settled: idempotent repeat
	}
	if err := move(ctx, evt.OrderRef); err != nil {
		return err
	}
	updated, err := s.repo.MarkStatus(ctx, evt.IntentID, status)
	if err != nil {
		return err
	}
	view := toPublic(updated)
	if status == public.StatusSucceeded {
		s.notify.PaymentSucceeded(ctx, view)
	} else {
		s.notify.PaymentFailed(ctx, view)
	}
	return nil
}
