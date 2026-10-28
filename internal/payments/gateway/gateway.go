// Package gateway isolates the Stripe SDK behind a small interface.
// Tests substitute a fake; the app wires the Stripe implementation once.
// Nothing outside the payments module imports this package.
package gateway

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/paymentintent"
	"github.com/stripe/stripe-go/v82/webhook"
)

// Intent is the created charge handle.
type Intent struct {
	ID           string
	ClientSecret string
}

type CheckoutSession struct {
	ID        string
	URL       string
	ExpiresAt int64
}

// WebhookEvent is the verified outcome of one Stripe event.
type WebhookEvent struct {
	Type          string
	IntentID      string
	OrderRef      string
	CheckoutID    string
	PaymentStatus string
}

// Gateway talks to the payment provider.
type Gateway interface {
	CreateIntent(ctx context.Context, amountCents int64, currency, orderID string) (Intent, error)
	CreateCheckoutSession(ctx context.Context, amountCents int64, currency, orderID, successURL, cancelURL string) (CheckoutSession, error)
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

func (g *StripeGateway) CreateCheckoutSession(_ context.Context, amountCents int64, currency, orderID, successURL, cancelURL string) (CheckoutSession, error) {
	params := &stripe.CheckoutSessionParams{
		Mode:              stripe.String(string(stripe.CheckoutSessionModePayment)),
		SuccessURL:        stripe.String(successURL),
		CancelURL:         stripe.String(cancelURL),
		ClientReferenceID: stripe.String(orderID),
		Metadata:          map[string]string{"order_id": orderID},
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency:   stripe.String(currency),
					UnitAmount: stripe.Int64(amountCents),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String("Malawi Store order " + orderID),
					},
				},
				Quantity: stripe.Int64(1),
			},
		},
	}
	cs, err := session.New(params)
	if err != nil {
		return CheckoutSession{}, fmt.Errorf("stripe create checkout session: %w", err)
	}
	return CheckoutSession{ID: cs.ID, URL: cs.URL, ExpiresAt: cs.ExpiresAt}, nil
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
	if event.Type == "checkout.session.completed" || event.Type == "checkout.session.async_payment_succeeded" || event.Type == "checkout.session.async_payment_failed" {
		var cs stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &cs); err != nil {
			return WebhookEvent{}, fmt.Errorf("stripe parse checkout session: %w", err)
		}
		return WebhookEvent{Type: string(event.Type), CheckoutID: cs.ID, OrderRef: cs.Metadata["order_id"], PaymentStatus: string(cs.PaymentStatus)}, nil
	}
	var pi stripe.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
		return WebhookEvent{}, fmt.Errorf("stripe parse intent: %w", err)
	}
	return WebhookEvent{Type: string(event.Type), IntentID: pi.ID, OrderRef: pi.Metadata["order_id"]}, nil
}
