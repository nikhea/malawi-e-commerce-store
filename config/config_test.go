package config

import (
	"strings"
	"testing"
)

func TestLoadRequiresStripeWebhookSecret(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgresql://localhost/store")
	t.Setenv("JWT_SECRET", strings.Repeat("s", 32))
	t.Setenv("STRIPE_WEBHOOKS_SIGNING_SECRET", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error when the webhook secret is empty")
	}
	if !strings.Contains(err.Error(), "STRIPE_WEBHOOKS_SIGNING_SECRET") {
		t.Fatalf("error should name the missing key, got %v", err)
	}
}

func TestLoadAcceptsWebhookSecret(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgresql://localhost/store")
	t.Setenv("JWT_SECRET", strings.Repeat("s", 32))
	t.Setenv("STRIPE_WEBHOOKS_SIGNING_SECRET", "whsec_test")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.StripeWebhookSecret != "whsec_test" {
		t.Fatalf("secret not carried through: %q", cfg.StripeWebhookSecret)
	}
}
