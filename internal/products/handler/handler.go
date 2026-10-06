package handler

import (
	"io"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	mediapublic "github.com/nikhea/malawi-e-commerce-store/internal/media/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/products/dto"
	"github.com/nikhea/malawi-e-commerce-store/internal/products/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	"github.com/nikhea/malawi-e-commerce-store/pkg/response"
)

// maxImageBytes caps a single upload at 10 MiB (mirrors the worker cap).
const maxImageBytes = 10 << 20

// Handler is thin: bind input, call the service, write the envelope.
type Handler struct {
	svc   public.Service
	media mediapublic.Service
}

func New(svc public.Service, media mediapublic.Service) *Handler {
	return &Handler{svc: svc, media: media}
}

// List godoc
// @Summary List products
// @Description Lightweight catalog rows (no variants — fetch Detail per product).
// @Tags products
// @Produce json
// @Param category_id query string false "Filter by category"
// @Param active_only query bool false "Hide inactive (default true)"
// @Param limit query int false "Page size (default 20, max 100)"
// @Param offset query int false "Page offset"
// @Success 200 {array} public.Product
// @Router /products [get]
func (h *Handler) List(c *gin.Context) {
	var q dto.ListProductsQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Error(c, apperr.Validation("invalid query"))
		return
	}
	activeOnly := true
	if q.ActiveOnly != nil {
		activeOnly = *q.ActiveOnly
	}
	products, err := h.svc.List(c.Request.Context(), public.ListFilter{
		CategoryID: q.CategoryID, ActiveOnly: activeOnly, Limit: q.Limit, Offset: q.Offset,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, products)
}

// GetByID godoc
// @Summary Get product detail by ID
// @Description Product + its variants (the storefront product page).
// @Tags products
// @Produce json
// @Param id path string true "Product ID"
// @Success 200 {object} public.Detail
// @Failure 404 {object} response.ErrorResponse
// @Router /products/{id} [get]
func (h *Handler) GetByID(c *gin.Context) {
	d, err := h.svc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, d)
}

// GetBySlug godoc
// @Summary Get product detail by slug
// @Description Pretty-URL lookup used by the storefront.
// @Tags products
// @Produce json
// @Param slug path string true "Product slug"
// @Success 200 {object} public.Detail
// @Failure 404 {object} response.ErrorResponse
// @Router /products/slug/{slug} [get]
func (h *Handler) GetBySlug(c *gin.Context) {
	d, err := h.svc.GetBySlug(c.Request.Context(), c.Param("slug"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, d)
}

// Create godoc
// @Summary Create a product (admin)
// @Tags admin
// @Accept json
// @Produce json
// @Param body body dto.CreateProductRequest true "Product"
// @Success 201 {object} public.Product
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /admin/products [post]
func (h *Handler) Create(c *gin.Context) {
	var req dto.CreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.Validation("name is required"))
		return
	}
	p, err := h.svc.Create(c.Request.Context(), public.CreateProductInput{
		CategoryID: req.CategoryID, Name: req.Name, Slug: req.Slug,
		Description: req.Description, PriceCents: req.PriceCents,
		Currency: req.Currency, ImageURL: req.ImageURL,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, p)
}

// Update godoc
// @Summary Update a product (admin)
// @Description Slug is immutable. Pass clear_category to unfile.
// @Tags admin
// @Accept json
// @Produce json
// @Param id path string true "Product ID"
// @Param body body dto.UpdateProductRequest true "Fields"
// @Success 200 {object} public.Product
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /admin/products/{id} [patch]
func (h *Handler) Update(c *gin.Context) {
	var req dto.UpdateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.Validation("invalid body"))
		return
	}
	p, err := h.svc.Update(c.Request.Context(), c.Param("id"), public.UpdateProductInput{
		CategoryID: req.CategoryID, Name: req.Name, Description: req.Description,
		PriceCents: req.PriceCents, Currency: req.Currency, IsActive: req.IsActive,
		ClearCategory: req.ClearCategory,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, p)
}

// Delete godoc
// @Summary Delete a product (admin)
// @Description Hard delete — variants cascade. Prefer deactivating.
// @Tags admin
// @Produce json
// @Param id path string true "Product ID"
// @Success 204
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /admin/products/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

// UploadImage godoc
// @Summary Upload a product image (admin)
// @Description Multipart upload via the River queue (202 immediately).
// @Tags admin
// @Accept multipart/form-data
// @Produce json
// @Param id path string true "Product ID"
// @Param file formData file true "Image (max 10 MiB)"
// @Success 202 {object} map[string]string
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /admin/products/{id}/image [post]
func (h *Handler) UploadImage(c *gin.Context) {
	id := c.Param("id")
	if _, err := h.svc.GetByID(c.Request.Context(), id); err != nil {
		response.Error(c, err)
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.Error(c, apperr.Validation("image file is required"))
		return
	}
	defer file.Close()
	if header.Size > maxImageBytes {
		response.Error(c, apperr.Validation("image exceeds 10 MiB"))
		return
	}

	tmp, err := os.CreateTemp("", "prodimg-*"+filepath.Ext(header.Filename))
	if err != nil {
		response.Error(c, apperr.Internal(err))
		return
	}
	tmpPath := tmp.Name()
	if _, err := io.Copy(tmp, file); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		response.Error(c, apperr.Internal(err))
		return
	}
	tmp.Close()

	if err := h.media.Enqueue(c.Request.Context(), mediapublic.EnqueueInput{
		OwnerType: mediapublic.OwnerProduct,
		OwnerID:   id,
		Filename:  header.Filename,
		Folder:    "malawi-store/products",
		TempPath:  tmpPath,
	}); err != nil {
		os.Remove(tmpPath)
		response.Error(c, err)
		return
	}
	response.Accepted(c, gin.H{"status": "queued"})
}
