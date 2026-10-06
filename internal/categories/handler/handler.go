package handler

import (
	"io"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/internal/categories/dto"
	"github.com/nikhea/malawi-e-commerce-store/internal/categories/public"
	mediapublic "github.com/nikhea/malawi-e-commerce-store/internal/media/public"
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
// @Summary List categories
// @Description Public storefront taxonomy, flat and ordered by name.
// @Tags categories
// @Produce json
// @Success 200 {array} public.Category
// @Router /categories [get]
func (h *Handler) List(c *gin.Context) {
	cats, err := h.svc.List(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, cats)
}

// GetByID godoc
// @Summary Get category by ID
// @Tags categories
// @Produce json
// @Param id path string true "Category ID"
// @Success 200 {object} public.Category
// @Failure 404 {object} response.ErrorResponse
// @Router /categories/{id} [get]
func (h *Handler) GetByID(c *gin.Context) {
	cat, err := h.svc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, cat)
}

// GetBySlug godoc
// @Summary Get category by slug
// @Description Pretty-URL lookup used by the storefront.
// @Tags categories
// @Produce json
// @Param slug path string true "Category slug"
// @Success 200 {object} public.Category
// @Failure 404 {object} response.ErrorResponse
// @Router /categories/slug/{slug} [get]
func (h *Handler) GetBySlug(c *gin.Context) {
	cat, err := h.svc.GetBySlug(c.Request.Context(), c.Param("slug"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, cat)
}

// Create godoc
// @Summary Create a category (admin)
// @Tags admin
// @Accept json
// @Produce json
// @Param body body dto.CreateCategoryRequest true "Category"
// @Success 201 {object} public.Category
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /admin/categories [post]
func (h *Handler) Create(c *gin.Context) {
	var req dto.CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.Validation("name is required"))
		return
	}
	cat, err := h.svc.Create(c.Request.Context(), public.CreateCategoryInput{
		Name: req.Name, Slug: req.Slug,
		Description: req.Description, ParentID: req.ParentID,
		ImageURL: req.ImageURL,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, cat)
}

// Update godoc
// @Summary Update a category (admin)
// @Description Slug is immutable. Pass clear_parent to detach to root.
// @Tags admin
// @Accept json
// @Produce json
// @Param id path string true "Category ID"
// @Param body body dto.UpdateCategoryRequest true "Fields"
// @Success 200 {object} public.Category
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /admin/categories/{id} [patch]
func (h *Handler) Update(c *gin.Context) {
	var req dto.UpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.Validation("invalid body"))
		return
	}
	cat, err := h.svc.Update(c.Request.Context(), c.Param("id"), public.UpdateCategoryInput{
		Name: req.Name, Description: req.Description,
		ParentID: req.ParentID, ClearParent: req.ClearParent,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, cat)
}

// UploadImage godoc
// @Summary Upload a category image (admin)
// @Description Multipart upload. Queued in River, processed to Cloudinary
// @Description async — answers 202 immediately, image appears once the
// @Description worker finishes. Omit the file entirely if no image.
// @Tags admin
// @Accept multipart/form-data
// @Produce json
// @Param id path string true "Category ID"
// @Param file formData file true "Image (max 10 MiB)"
// @Success 202 {object} map[string]string
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /admin/categories/{id}/image [post]
func (h *Handler) UploadImage(c *gin.Context) {
	id := c.Param("id")
	// Fail fast: 404 before touching disk or queue.
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

	tmp, err := os.CreateTemp("", "catimg-*"+filepath.Ext(header.Filename))
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
		OwnerType: mediapublic.OwnerCategory,
		OwnerID:   id,
		Filename:  header.Filename,
		Folder:    "malawi-store/categories",
		TempPath:  tmpPath,
	}); err != nil {
		os.Remove(tmpPath)
		response.Error(c, err)
		return
	}
	response.Accepted(c, gin.H{"status": "queued"})
}

// Delete godoc
// @Summary Delete a category (admin)
// @Description Children become roots (SET NULL), they are not deleted.
// @Tags admin
// @Produce json
// @Param id path string true "Category ID"
// @Success 204
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /admin/categories/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}
