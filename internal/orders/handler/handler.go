package handler

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/internal/orders/dto"
	"github.com/nikhea/malawi-e-commerce-store/internal/orders/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	"github.com/nikhea/malawi-e-commerce-store/pkg/middleware"
	"github.com/nikhea/malawi-e-commerce-store/pkg/response"
)

// Handler is thin: identity from the JWT, call the service, write the
// envelope. UserID never comes from the client.
type Handler struct {
	svc public.Service
}

func New(svc public.Service) *Handler {
	return &Handler{svc: svc}
}

// Checkout godoc
// @Summary Check out my cart
// @Description Freezes the cart into a pending order, reserves stock, clears the cart.
// @Tags orders
// @Produce json
// @Success 201 {object} public.Order
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /orders/checkout [post]
func (h *Handler) Checkout(c *gin.Context) {
	order, err := h.svc.Checkout(c.Request.Context(), middleware.UserIDOf(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, order)
}

// ListMine godoc
// @Summary List my orders
// @Description Caller's orders with paging and filters, newest first.
// @Description Sends {"data": [orders], "meta": {page, limit, …}}.
// @Tags orders
// @Produce json
// @Param status query string false "pending|paid|cancelled"
// @Param currency query string false "Filter by currency (e.g. USD)"
// @Param created_after query string false "Created at or after (RFC3339 or YYYY-MM-DD)"
// @Param created_before query string false "Created at or before (RFC3339 or YYYY-MM-DD)"
// @Param min_total_cents query int false "Minimum subtotal in minor units"
// @Param max_total_cents query int false "Maximum subtotal in minor units"
// @Param sort query string false "newest|oldest|total_asc|total_desc"
// @Param page query int false "1-based page (wins over offset)"
// @Param limit query int false "Page size (default 12, max 100)"
// @Param offset query int false "Page offset"
// @Success 200 {object} public.OrderListResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /orders [get]
func (h *Handler) ListMine(c *gin.Context) {
	var q dto.ListOrdersQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Error(c, apperr.Validation("invalid query"))
		return
	}
	after, err := parseDateParam(q.CreatedAfter, false)
	if err != nil {
		response.Error(c, apperr.Validation("created_after must be RFC3339 or YYYY-MM-DD"))
		return
	}
	before, err := parseDateParam(q.CreatedBefore, true)
	if err != nil {
		response.Error(c, apperr.Validation("created_before must be RFC3339 or YYYY-MM-DD"))
		return
	}
	page, err := h.svc.ListMine(c.Request.Context(), middleware.UserIDOf(c), public.ListFilter{
		Status: q.Status, Currency: q.Currency,
		CreatedAfter: after, CreatedBefore: before,
		MinTotalCents: q.MinTotalCents, MaxTotalCents: q.MaxTotalCents,
		Sort: q.Sort, Page: q.Page, Limit: q.Limit, Offset: q.Offset,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OKMeta(c, page.Items, page.Meta)
}

// parseDateParam accepts RFC3339 timestamps and plain calendar dates.
// A date-only created_before expands to the end of that day.
func parseDateParam(raw string, endOfDay bool) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return &t, nil
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, err
	}
	if endOfDay {
		t = t.Add(24*time.Hour - time.Nanosecond)
	}
	return &t, nil
}

// GetByID godoc
// @Summary Get my order by ID
// @Tags orders
// @Produce json
// @Param id path string true "Order ID"
// @Success 200 {object} public.Order
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /orders/{id} [get]
func (h *Handler) GetByID(c *gin.Context) {
	order, err := h.svc.GetByID(c.Request.Context(), middleware.UserIDOf(c), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, order)
}

// Cancel godoc
// @Summary Cancel my pending order
// @Description Releases the stock hold. Pending orders only.
// @Tags orders
// @Produce json
// @Param id path string true "Order ID"
// @Success 200 {object} public.Order
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /orders/{id}/cancel [post]
func (h *Handler) Cancel(c *gin.Context) {
	order, err := h.svc.Cancel(c.Request.Context(), middleware.UserIDOf(c), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, order)
}
