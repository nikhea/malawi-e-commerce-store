package handler

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/internal/payments/dto"
	"github.com/nikhea/malawi-e-commerce-store/internal/payments/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	"github.com/nikhea/malawi-e-commerce-store/pkg/middleware"
	"github.com/nikhea/malawi-e-commerce-store/pkg/response"
)

// maxWebhookBytes caps Stripe payloads at 1 MiB (events are small JSON).
const maxWebhookBytes = 1 << 20

// Handler is thin: bind input, call the service, write the envelope.
// The webhook path reads the RAW body — signature verification fails on
// even one altered byte, so no binding happens before Verify.
type Handler struct {
	svc public.Service
}

func New(svc public.Service) *Handler {
	return &Handler{svc: svc}
}

// CreateIntent godoc
// @Summary Create a payment intent
// @Description Validates ownership, converts the MWK total, reuses pending intents.
// @Tags payments
// @Accept json
// @Produce json
// @Param body body dto.CreateIntentRequest true "Order"
// @Success 201 {object} public.Payment
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /payments/intents [post]
func (h *Handler) CreateIntent(c *gin.Context) {
	var req dto.CreateIntentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.Validation("order_id is required"))
		return
	}
	payment, err := h.svc.CreateIntent(c.Request.Context(), middleware.UserIDOf(c), req.OrderID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, payment)
}

// Webhook godoc
// @Summary Stripe webhook
// @Description Verifies the signature, settles the order. No JWT — the
// @Description signature IS the authentication. Mounted on the open group.
// @Tags webhooks
// @Accept */*
// @Produce json
// @Param Stripe-Signature header string true "Stripe signature"
// @Success 200 {object} map[string]string
// @Failure 400 {object} response.ErrorResponse
// @Router /webhooks/stripe [post]
func (h *Handler) Webhook(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxWebhookBytes)
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.Error(c, apperr.Validation("unreadable payload"))
		return
	}
	if err := h.svc.HandleWebhook(c.Request.Context(), payload, c.GetHeader("Stripe-Signature")); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"status": "received"})
}
