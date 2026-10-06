// Package gateway isolates the Stripe SDK behind a small interface.
// Tests substitute a fake; the app wires the Stripe implementation once.
// Nothing outside the payments module imports this package.
package gateway

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/paymentintent"
	"github.com/stripe/stripe-go/v82/webhook"
)

// Intent is the created charge handle.
type Intent struct {
	ID           string
	ClientSecret string
}

// WebhookEvent is the verified outcome of one Stripe event.
type WebhookEvent struct {
	Type     string
	IntentID string
	OrderRef string
}

// Gateway talks to the payment provider.
type Gateway interface {
	CreateIntent(ctx context.Context, amountCents int64, currency, orderID string) (Intent, error)
	VerifyWebhook(payload []byte, sigHeader, webhookSecret string) (WebhookEvent, error)
}

// StripeGateway is the production Gateway. stripe.Key is process-global
// in stripe-go: one backend per monolith, set once in the constructor.
type StripeGateway struct{}

func NewStripeGateway(secretKey string) *StripeGateway {
	stripe.Key = secretKey
	return &StripeGateway{}
}

func (g *StripeGateway) CreateIntent(_ context.Context, amountCents int64, currency, orderID string) (Intent, error) {
	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(amountCents),
		Currency: stripe.String(currency),
		Metadata: map[string]string{"order_id": orderID},
		// Automatic methods let Stripe pick card/mobile-money rails.
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled:        stripe.Bool(true),
			AllowRedirects: stripe.String("never"),
		},
	}
	pi, err := paymentintent.New(params)
	if err != nil {
		return Intent{}, fmt.Errorf("stripe create intent: %w", err)
	}
	return Intent{ID: pi.ID, ClientSecret: pi.ClientSecret}, nil
}

func (g *StripeGateway) VerifyWebhook(payload []byte, sigHeader, webhookSecret string) (WebhookEvent, error) {
	// Strict ConstructEvent: rejects version-mismatched payloads instead
	// of mis-parsing them. Consequence: the Stripe dashboard endpoint
	// MUST be created with stripe-go's pinned API version (see the SDK
	// mismatch error if this ever fails in production).
	event, err := webhook.ConstructEvent(payload, sigHeader, webhookSecret)
	if err != nil {
		return WebhookEvent{}, fmt.Errorf("stripe verify webhook: %w", err)
	}
	var pi stripe.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
		return WebhookEvent{}, fmt.Errorf("stripe parse intent: %w", err)
	}
	return WebhookEvent{
		Type:     string(event.Type),
		IntentID: pi.ID,
		OrderRef: pi.Metadata["order_id"],
	}, nil
}
