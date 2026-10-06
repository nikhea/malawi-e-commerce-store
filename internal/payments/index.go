package payments

import (
	"github.com/jackc/pgx/v5/pgxpool"
	orderspublic "github.com/nikhea/malawi-e-commerce-store/internal/orders/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/payments/gateway"
	"github.com/nikhea/malawi-e-commerce-store/internal/payments/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/payments/repository"
	paymentservice "github.com/nikhea/malawi-e-commerce-store/internal/payments/service"
)

// Wire builds the payments module around orders (settle moves), the
// Stripe gateway, and a notifier for outcome fan-out (production:
// *notify.Service). The gateway is constructed at the wiring site (it
// holds the secret key); the service takes the FX rate + webhook secret
// via Config. Single entry point.
func Wire(pool *pgxpool.Pool, orders orderspublic.Service, gw gateway.Gateway, notify paymentservice.Notifier, webhookSecret string, fxRate float64) public.Service {
	return paymentservice.NewService(
		repository.NewPostgres(pool), orders, gw, notify,
		paymentservice.Config{WebhookSecret: webhookSecret, FXMWKPerUSD: fxRate},
	)
}
