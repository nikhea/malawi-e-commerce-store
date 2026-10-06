package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/internal/orders/public"
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
// @Tags orders
// @Produce json
// @Success 200 {array} public.Order
// @Failure 401 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /orders [get]
func (h *Handler) ListMine(c *gin.Context) {
	orders, err := h.svc.ListMine(c.Request.Context(), middleware.UserIDOf(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, orders)
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
