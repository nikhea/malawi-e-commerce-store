package config

import (
	"strings"
	"testing"
)

func baseEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgresql://localhost/store")
	t.Setenv("JWT_SECRET", strings.Repeat("s", 32))
	t.Setenv("STRIPE_WEBHOOKS_SIGNING_SECRET", "whsec_test")
}

func TestLoadRequiresStripeWebhookSecret(t *testing.T) {
	baseEnv(t)
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
	baseEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.StripeWebhookSecret != "whsec_test" {
		t.Fatalf("secret not carried through: %q", cfg.StripeWebhookSecret)
	}
}

func TestLoadRequiresLongJWTSecret(t *testing.T) {
	baseEnv(t)
	t.Setenv("JWT_SECRET", "short")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "32 bytes") {
		t.Fatalf("expected a length error, got %v", err)
	}
}

func TestLoadRejectsNonPositiveFX(t *testing.T) {
	baseEnv(t)
	t.Setenv("FX_MWK_PER_USD", "0")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "FX_MWK_PER_USD") {
		t.Fatalf("expected an FX error, got %v", err)
	}
	// The old code used %w with a nil error and printed "%!w(<nil>)".
	if strings.Contains(err.Error(), "%!w") {
		t.Fatalf("malformed format verb leaked into the message: %v", err)
	}
}

func TestEmailRelayDefaults(t *testing.T) {
	baseEnv(t)
	t.Setenv("EMAIL_HOST", "")
	t.Setenv("EMAIL_PORT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.EmailHost != "smtp.gmail.com" || cfg.EmailPort != 587 {
		t.Fatalf("unexpected relay defaults: %s:%d", cfg.EmailHost, cfg.EmailPort)
	}
}
