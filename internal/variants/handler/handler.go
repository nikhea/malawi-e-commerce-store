package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/internal/variants/dto"
	"github.com/nikhea/malawi-e-commerce-store/internal/variants/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	"github.com/nikhea/malawi-e-commerce-store/pkg/bulk"
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

// BulkCreate godoc
// @Summary Create variants in bulk (admin)
// @Description Per-item results in a 200 envelope. The product must exist
// @Description (whole-request 404). SKUs are uppercased like single create;
// @Description duplicate SKUs in the batch reject the request, while a DB SKU
// @Description collision is a per-item CONFLICT. dryRun projects without
// @Description writing; atomic inserts every SKU in one transaction.
// @Tags admin
// @Accept json
// @Produce json
// @Param id path string true "Product ID"
// @Param Idempotency-Key header string false "Replay-safe request key (24h)"
// @Param body body dto.BulkCreateVariantsRequest true "SKUs"
// @Success 200 {object} dto.BulkCreateVariantsResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /products/{id}/variants/bulk [post]
func (h *Handler) BulkCreate(c *gin.Context) {
	var req dto.BulkCreateVariantsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.Validation("items is required"))
		return
	}
	productID := c.Param("id")
	items := make([]public.CreateVariantInput, 0, len(req.Items))
	for _, in := range req.Items {
		items = append(items, public.CreateVariantInput{
			ProductID: productID, Name: in.Name, SKU: in.SKU, PriceCents: in.PriceCents,
		})
	}
	started := time.Now()
	results, err := h.svc.BulkCreate(c.Request.Context(), productID, items, req.Atomic, req.DryRun)
	if err != nil {
		response.Error(c, err)
		return
	}
	summary := bulk.Summary(results)
	bulk.Log(c, "variants.bulk_create", started, summary)
	response.OK(c, dto.BulkCreateVariantsResponse{Results: results, Summary: summary})
}

// BulkUpdate godoc
// @Summary Update variants in bulk (admin)
// @Description One partial patch per variant; product_id and sku stay
// @Description immutable. An empty patch is a per-item validation error.
// @Description Duplicate ids reject the whole request. Availability is read
// @Description before writes; dryRun projects without writing.
// @Tags admin
// @Accept json
// @Produce json
// @Param Idempotency-Key header string false "Replay-safe request key (24h)"
// @Param body body dto.BulkUpdateVariantsRequest true "Patches"
// @Success 200 {object} dto.BulkUpdateVariantsResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /admin/variants/bulk [patch]
func (h *Handler) BulkUpdate(c *gin.Context) {
	var req dto.BulkUpdateVariantsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.Validation("updates is required"))
		return
	}
	items := make([]public.BulkUpdateItem, 0, len(req.Updates))
	for _, u := range req.Updates {
		items = append(items, public.BulkUpdateItem{
			ID: u.ID,
			Patch: public.UpdateVariantInput{
				Name: u.Patch.Name, PriceCents: u.Patch.PriceCents, IsActive: u.Patch.IsActive,
			},
		})
	}
	started := time.Now()
	results, err := h.svc.BulkUpdate(c.Request.Context(), items, req.Atomic, req.DryRun)
	if err != nil {
		response.Error(c, err)
		return
	}
	summary := bulk.Summary(results)
	bulk.Log(c, "variants.bulk_update", started, summary)
	response.OK(c, dto.BulkUpdateVariantsResponse{Results: results, Summary: summary})
}
