package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/internal/cart/dto"
	"github.com/nikhea/malawi-e-commerce-store/internal/cart/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	"github.com/nikhea/malawi-e-commerce-store/pkg/middleware"
	"github.com/nikhea/malawi-e-commerce-store/pkg/response"
)

// Handler is thin: identity from the JWT, bind input, call service.
// UserID never comes from the client — middleware.UserIDOf only.
type Handler struct {
	svc public.Service
}

func New(svc public.Service) *Handler {
	return &Handler{svc: svc}
}

// Get godoc
// @Summary Get my cart
// @Description One page of active-cart lines with snapshot prices. Subtotals
// @Description always cover the whole cart. Sends {"data": Cart, "meta": …}.
// @Tags cart
// @Produce json
// @Param q query string false "Search line name or SKU"
// @Param product_id query string false "Only lines for this product"
// @Param variant_id query string false "Only lines for this variant"
// @Param sort query string false "oldest|newest|name_asc|name_desc|price_asc|price_desc|qty_asc|qty_desc"
// @Param page query int false "1-based page (wins over offset)"
// @Param limit query int false "Page size (default 12, max 100)"
// @Param offset query int false "Page offset"
// @Success 200 {object} public.CartListResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /cart [get]
func (h *Handler) Get(c *gin.Context) {
	var q dto.ListCartQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Error(c, apperr.Validation("invalid query"))
		return
	}
	page, err := h.svc.List(c.Request.Context(), middleware.UserIDOf(c), public.ListFilter{
		Query: q.Q, ProductID: q.ProductID, VariantID: q.VariantID,
		Sort: q.Sort, Page: q.Page, Limit: q.Limit, Offset: q.Offset,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OKMeta(c, page.Cart, page.Meta)
}

// AddItem godoc
// @Summary Add an item
// @Description Validates against the catalog and snapshots the price.
// @Tags cart
// @Accept json
// @Produce json
// @Param body body dto.AddItemRequest true "Line"
// @Success 200 {object} public.Cart
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /cart/items [post]
func (h *Handler) AddItem(c *gin.Context) {
	var req dto.AddItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.Validation("product_id and qty are required"))
		return
	}
	cart, err := h.svc.AddItem(c.Request.Context(), public.AddItemInput{
		UserID:    middleware.UserIDOf(c),
		ProductID: req.ProductID, VariantID: req.VariantID, Qty: req.Qty,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, cart)
}

// SetQty godoc
// @Summary Change a line's quantity
// @Description Qty 0 removes the line.
// @Tags cart
// @Accept json
// @Produce json
// @Param id path string true "Item ID"
// @Param body body dto.SetQtyRequest true "Qty"
// @Success 200 {object} public.Cart
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /cart/items/{id} [patch]
func (h *Handler) SetQty(c *gin.Context) {
	var req dto.SetQtyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.Validation("qty is required"))
		return
	}
	cart, err := h.svc.SetQty(c.Request.Context(), middleware.UserIDOf(c), c.Param("id"), req.Qty)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, cart)
}

// RemoveItem godoc
// @Summary Remove a line
// @Tags cart
// @Produce json
// @Param id path string true "Item ID"
// @Success 200 {object} public.Cart
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /cart/items/{id} [delete]
func (h *Handler) RemoveItem(c *gin.Context) {
	cart, err := h.svc.RemoveItem(c.Request.Context(), middleware.UserIDOf(c), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, cart)
}

// Clear godoc
// @Summary Empty the cart
// @Tags cart
// @Produce json
// @Success 204
// @Failure 401 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /cart [delete]
func (h *Handler) Clear(c *gin.Context) {
	if err := h.svc.Clear(c.Request.Context(), middleware.UserIDOf(c)); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}
