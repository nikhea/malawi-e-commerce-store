package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/internal/variants/dto"
	"github.com/nikhea/malawi-e-commerce-store/internal/variants/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	"github.com/nikhea/malawi-e-commerce-store/pkg/response"
)

// Handler is thin: bind input, call the service, write the envelope.
type Handler struct {
	svc public.Service
}

func New(svc public.Service) *Handler {
	return &Handler{svc: svc}
}

// ListByProduct godoc
// @Summary List a product's variants
// @Description Public SKU list for the storefront product page.
// @Tags products
// @Produce json
// @Param id path string true "Product ID"
// @Success 200 {array} public.Variant
// @Router /products/{id}/variants [get]
func (h *Handler) ListByProduct(c *gin.Context) {
	variants, err := h.svc.ListByProduct(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, variants)
}

// Create godoc
// @Summary Create a variant (admin)
// @Tags admin
// @Accept json
// @Produce json
// @Param id path string true "Product ID"
// @Param body body dto.CreateVariantRequest true "SKU"
// @Success 201 {object} public.Variant
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /products/{id}/variants [post]
func (h *Handler) Create(c *gin.Context) {
	var req dto.CreateVariantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.Validation("name, sku and price are required"))
		return
	}
	v, err := h.svc.Create(c.Request.Context(), public.CreateVariantInput{
		ProductID: c.Param("id"), Name: req.Name, SKU: req.SKU, PriceCents: req.PriceCents,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, v)
}

// Update godoc
// @Summary Update a variant (admin)
// @Description product_id and sku are immutable.
// @Tags admin
// @Accept json
// @Produce json
// @Param id path string true "Variant ID"
// @Param body body dto.UpdateVariantRequest true "Fields"
// @Success 200 {object} public.Variant
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /admin/variants/{id} [patch]
func (h *Handler) Update(c *gin.Context) {
	var req dto.UpdateVariantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.Validation("invalid body"))
		return
	}
	v, err := h.svc.Update(c.Request.Context(), c.Param("id"), public.UpdateVariantInput{
		Name: req.Name, PriceCents: req.PriceCents, IsActive: req.IsActive,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, v)
}

// Delete godoc
// @Summary Delete a variant (admin)
// @Tags admin
// @Produce json
// @Param id path string true "Variant ID"
// @Success 204
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /admin/variants/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}
