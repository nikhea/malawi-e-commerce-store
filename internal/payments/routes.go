package payments

import (
	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/internal/payments/handler"
	"github.com/nikhea/malawi-e-commerce-store/internal/payments/public"
)

// RegisterRoutes maps the payments HTTP surface. Split trust: intents are
// JWT-protected (money needs identity); the webhook is OPEN because
// Stripe can't log in — the HMAC signature is its authentication.
// Route table only, no logic.
func RegisterRoutes(open, protected *gin.RouterGroup, svc public.Service) {
	h := handler.New(svc)

	webhooks := open.Group("/webhooks")
	webhooks.POST("/stripe", h.Webhook)

	payments := protected.Group("/payments")
	payments.POST("/intents", h.CreateIntent)
}
