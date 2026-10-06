package test

import (
	"context"
	"errors"
	"testing"

	orderspublic "github.com/nikhea/malawi-e-commerce-store/internal/orders/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/payments/gateway"
	"github.com/nikhea/malawi-e-commerce-store/internal/payments/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/payments/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/payments/service"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	"github.com/nikhea/malawi-e-commerce-store/pkg/events"
)

// fakeRepo is an in-memory payments ledger.
type fakeRepo struct {
	byOrder  map[string]model.Payment
	byIntent map[string]string // intent → order
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{byOrder: map[string]model.Payment{}, byIntent: map[string]string{}}
}

func (f *fakeRepo) Create(_ context.Context, p model.Payment) (model.Payment, error) {
	p.ID = "pay-" + p.OrderID
	p.Status = public.StatusPending
	f.byOrder[p.OrderID] = p
	f.byIntent[p.StripeIntentID] = p.OrderID
	return p, nil
}

func (f *fakeRepo) GetByOrder(_ context.Context, orderID string) (model.Payment, error) {
	p, ok := f.byOrder[orderID]
	if !ok {
		return model.Payment{}, apperr.NotFound("payment not found")
	}
	return p, nil
}

func (f *fakeRepo) GetByIntent(_ context.Context, intentID string) (model.Payment, error) {
	orderID, ok := f.byIntent[intentID]
	if !ok {
		return model.Payment{}, apperr.NotFound("payment not found")
	}
	return f.byOrder[orderID], nil
}

func (f *fakeRepo) MarkStatus(_ context.Context, intentID, status string) (model.Payment, error) {
	orderID, ok := f.byIntent[intentID]
	if !ok {
		return model.Payment{}, apperr.NotFound("payment not found")
	}
	p := f.byOrder[orderID]
	p.Status = status
	f.byOrder[orderID] = p
	return p, nil
}

// fakeOrders serves one pending order ("order-1", 85000 tambala) and one
// paid order ("order-paid").
type fakeOrders struct {
	paid      map[string]bool
	markPaid  []string
	cancelled []string
}

func newFakeOrders() *fakeOrders {
	return &fakeOrders{paid: map[string]bool{"order-paid": true}}
}

func (f *fakeOrders) order(id string) (orderspublic.Order, error) {
	switch id {
	case "order-1":
		return orderspublic.Order{ID: "order-1", UserID: "u1", Status: orderspublic.StatusPending, SubtotalCents: 850000, Currency: "MWK"}, nil
	case "order-paid":
		return orderspublic.Order{ID: "order-paid", UserID: "u1", Status: orderspublic.StatusPaid, SubtotalCents: 85000, Currency: "MWK"}, nil
	}
	return orderspublic.Order{}, apperr.NotFound("order not found")
}

func (f *fakeOrders) Checkout(context.Context, string) (orderspublic.Order, error) {
	return orderspublic.Order{}, nil
}
func (f *fakeOrders) GetByID(_ context.Context, userID, orderID string) (orderspublic.Order, error) {
	o, err := f.order(orderID)
	if err != nil || o.UserID != userID {
		return orderspublic.Order{}, apperr.NotFound("order not found")
	}
	return o, nil
}
func (f *fakeOrders) GetByRef(_ context.Context, orderID string) (orderspublic.Order, error) {
	return f.order(orderID)
}
func (f *fakeOrders) ListMine(context.Context, string) ([]orderspublic.Order, error) {
	return nil, nil
}
func (f *fakeOrders) Cancel(context.Context, string, string) (orderspublic.Order, error) {
	return orderspublic.Order{}, nil
}
func (f *fakeOrders) MarkPaid(_ context.Context, ref string) (orderspublic.Order, error) {
	o, err := f.order(ref)
	if err != nil {
		return orderspublic.Order{}, err
	}
	if o.Status == orderspublic.StatusPaid {
		return o, nil // idempotent
	}
	f.markPaid = append(f.markPaid, ref)
	f.paid[ref] = true
	o.Status = orderspublic.StatusPaid
	return o, nil
}
func (f *fakeOrders) CancelByRef(_ context.Context, ref string) error {
	f.cancelled = append(f.cancelled, ref)
	return nil
}

// fakeGateway returns canned intents and verifies by rule.
type fakeGateway struct {
	intents int
	verify  func(payload []byte, sig string) (gateway.WebhookEvent, error)
}

func (f *fakeGateway) CreateIntent(_ context.Context, amountCents int64, currency, orderID string) (gateway.Intent, error) {
	f.intents++
	return gateway.Intent{ID: "pi-test", ClientSecret: "secret-test"}, nil
}

func (f *fakeGateway) VerifyWebhook(payload []byte, sig, _ string) (gateway.WebhookEvent, error) {
	return f.verify(payload, sig)
}

type fixture struct {
	svc       public.Service
	repo      *fakeRepo
	orders    *fakeOrders
	gw        *fakeGateway
	published []string
}

func newFixture(verify func([]byte, string) (gateway.WebhookEvent, error)) *fixture {
	fx := &fixture{repo: newFakeRepo(), orders: newFakeOrders(), gw: &fakeGateway{}}
	fx.gw.verify = verify
	bus := events.New()
	bus.Subscribe(public.PaymentSucceeded, func(_ context.Context, e events.Event) { fx.published = append(fx.published, e.Name) })
	bus.Subscribe(public.PaymentFailed, func(_ context.Context, e events.Event) { fx.published = append(fx.published, e.Name) })
	fx.svc = service.NewService(fx.repo, fx.orders, fx.gw, bus, service.Config{WebhookSecret: "whsec", FXMWKPerUSD: 1700})
	return fx
}

func TestCreateIntent(t *testing.T) {
	ctx := context.Background()

	t.Run("converts MWK and creates", func(t *testing.T) {
		fx := newFixture(nil)
		// 850000 tambala (8500 MWK) / 1700 = $5.00 → 500 cents.
		p, err := fx.svc.CreateIntent(ctx, "u1", "order-1")
		if err != nil {
			t.Fatalf("intent: %v", err)
		}
		if p.AmountCents != 500 || p.Currency != "USD" || p.OrderAmountCents != 850000 {
			t.Fatalf("bad conversion: %+v", p)
		}
		if p.ClientSecret == "" || p.StripeIntentID == "" {
			t.Fatalf("missing stripe handle: %+v", p)
		}
	})

	t.Run("retry reuses pending intent", func(t *testing.T) {
		fx := newFixture(nil)
		first, _ := fx.svc.CreateIntent(ctx, "u1", "order-1")
		second, err := fx.svc.CreateIntent(ctx, "u1", "order-1")
		if err != nil {
			t.Fatalf("reuse: %v", err)
		}
		if first.StripeIntentID != second.StripeIntentID || fx.gw.intents != 1 {
			t.Fatalf("double intent created: %+v %+v", first, second)
		}
	})

	t.Run("foreign and non-pending orders rejected", func(t *testing.T) {
		fx := newFixture(nil)
		if _, err := fx.svc.CreateIntent(ctx, "intruder", "order-1"); apperr.CodeOf(err) != apperr.CodeNotFound {
			t.Fatalf("expected NOT_FOUND, got %v", err)
		}
		if _, err := fx.svc.CreateIntent(ctx, "u1", "order-paid"); apperr.CodeOf(err) != apperr.CodeConflict {
			t.Fatalf("expected CONFLICT, got %v", err)
		}
		if _, err := fx.svc.CreateIntent(ctx, "u1", "ghost"); apperr.CodeOf(err) != apperr.CodeNotFound {
			t.Fatalf("expected NOT_FOUND, got %v", err)
		}
	})
}

func TestHandleWebhook(t *testing.T) {
	ctx := context.Background()
	succeeded := gateway.WebhookEvent{Type: "payment_intent.succeeded", IntentID: "pi-test", OrderRef: "order-1"}
	failed := gateway.WebhookEvent{Type: "payment_intent.payment_failed", IntentID: "pi-test", OrderRef: "order-1"}

	t.Run("bad signature rejected", func(t *testing.T) {
		fx := newFixture(func([]byte, string) (gateway.WebhookEvent, error) {
			return gateway.WebhookEvent{}, errors.New("bad sig")
		})
		if err := fx.svc.HandleWebhook(ctx, []byte("{}"), "bad"); apperr.CodeOf(err) != apperr.CodeValidation {
			t.Fatalf("expected VALIDATION_ERROR, got %v", err)
		}
	})

	t.Run("succeeded settles and publishes", func(t *testing.T) {
		fx := newFixture(func([]byte, string) (gateway.WebhookEvent, error) { return succeeded, nil })
		if _, err := fx.svc.CreateIntent(ctx, "u1", "order-1"); err != nil {
			t.Fatalf("intent: %v", err)
		}
		if err := fx.svc.HandleWebhook(ctx, []byte("{}"), "sig"); err != nil {
			t.Fatalf("webhook: %v", err)
		}
		if len(fx.orders.markPaid) != 1 || len(fx.published) != 1 || fx.published[0] != public.PaymentSucceeded {
			t.Fatalf("settle wrong: %+v %v", fx.orders.markPaid, fx.published)
		}
		// Repeat is idempotent.
		if err := fx.svc.HandleWebhook(ctx, []byte("{}"), "sig"); err != nil {
			t.Fatalf("repeat: %v", err)
		}
	})

	t.Run("failed cancels and publishes", func(t *testing.T) {
		fx := newFixture(func([]byte, string) (gateway.WebhookEvent, error) { return failed, nil })
		if _, err := fx.svc.CreateIntent(ctx, "u1", "order-1"); err != nil {
			t.Fatalf("intent: %v", err)
		}
		if err := fx.svc.HandleWebhook(ctx, []byte("{}"), "sig"); err != nil {
			t.Fatalf("webhook: %v", err)
		}
		if len(fx.orders.cancelled) != 1 || fx.published[0] != public.PaymentFailed {
			t.Fatalf("cancel wrong: %+v %v", fx.orders.cancelled, fx.published)
		}
	})

	t.Run("unknown intent and type acknowledged", func(t *testing.T) {
		fx := newFixture(func([]byte, string) (gateway.WebhookEvent, error) {
			return gateway.WebhookEvent{Type: "payment_intent.succeeded", IntentID: "pi-stranger", OrderRef: "order-9"}, nil
		})
		if err := fx.svc.HandleWebhook(ctx, []byte("{}"), "sig"); err != nil {
			t.Fatalf("unknown intent should ack: %v", err)
		}
		fx2 := newFixture(func([]byte, string) (gateway.WebhookEvent, error) {
			return gateway.WebhookEvent{Type: "charge.refunded", IntentID: "pi-test"}, nil
		})
		if err := fx2.svc.HandleWebhook(ctx, []byte("{}"), "sig"); err != nil {
			t.Fatalf("unknown type should ack: %v", err)
		}
	})
}
