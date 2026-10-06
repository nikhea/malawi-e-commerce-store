package payments

import (
	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/internal/payments/handler"
	"github.com/nikhea/malawi-e-commerce-store/internal/payments/public"
)

// RegisterRoutes maps the payments HTTP surface. Split trust: intents are
// JWT-protected (money needs identity); the webhook group arrives
// pre-scoped by the wiring site (burst limiter — Stripe retries in
// floods). Route table only, no logic.
func RegisterRoutes(webhooks, protected *gin.RouterGroup, svc public.Service) {
	h := handler.New(svc)

	webhooks.POST("/stripe", h.Webhook)

	payments := protected.Group("/payments")
	payments.POST("/intents", h.CreateIntent)
}
