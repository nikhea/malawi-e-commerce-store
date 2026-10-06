package gateway_test

import (
	"testing"

	"github.com/nikhea/malawi-e-commerce-store/internal/payments/gateway"
	"github.com/stripe/stripe-go/v82/webhook"
)

// TestVerifyWebhookRealSignature exercises the REAL Stripe verification
// (HMAC + tolerance + payload parse) with a locally-signed payload — no
// network, no dashboard, no forged shortcuts.
func TestVerifyWebhookRealSignature(t *testing.T) {
	const secret = "whsec_test_local"
	const orderRef = "order-9"

	payload := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload: []byte(`{"id":"evt_test","object":"event","api_version":"2025-08-27.basil","type":"payment_intent.succeeded","data":{"object":{"id":"pi_test_123","object":"payment_intent","metadata":{"order_id":"` + orderRef + `"}}}}`),
		Secret:  secret,
	})

	gw := gateway.NewStripeGateway("sk_test_unused_here")
	evt, err := gw.VerifyWebhook(payload.Payload, payload.Header, secret)
	if err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
	if evt.Type != "payment_intent.succeeded" || evt.IntentID != "pi_test_123" || evt.OrderRef != orderRef {
		t.Fatalf("bad event: %+v", evt)
	}

	// Wrong secret must fail (this is what stops forgeries).
	if _, err := gw.VerifyWebhook(payload.Payload, payload.Header, "whsec_wrong"); err == nil {
		t.Fatal("wrong secret accepted")
	}
}
