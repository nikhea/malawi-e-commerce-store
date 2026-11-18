// Package handler keeps the dashboard HTTP surface thin: call the service,
// write the envelope. Authorization is enforced by the admin route group.
package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/internal/dashboard/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/response"
)

type Handler struct {
	svc public.Service
}

func New(svc public.Service) *Handler {
	return &Handler{svc: svc}
}

// Overview godoc
// @Summary Admin dashboard overview
// @Description Whole-store snapshot: catalog, customers, paying customers, order statuses, reservation/fulfillment states, stock health, payments, review summary and the top ordered/wishlisted/carted products.
// @Description Money stays grouped by currency; legacy MWK is never added to USD. Orders have no shipping states, so fulfillment is reported from inventory reservations.
// @Tags admin-dashboard
// @Produce json
// @Success 200 {object} public.OverviewResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /admin/dashboard/overview [get]
func (h *Handler) Overview(c *gin.Context) {
	overview, err := h.svc.Overview(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, overview)
}
