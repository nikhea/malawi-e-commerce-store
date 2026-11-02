package handler

import (
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	mediapublic "github.com/nikhea/malawi-e-commerce-store/internal/media/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/products/dto"
	"github.com/nikhea/malawi-e-commerce-store/internal/products/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	"github.com/nikhea/malawi-e-commerce-store/pkg/bulk"
	"github.com/nikhea/malawi-e-commerce-store/pkg/response"
)

// maxImageBytes caps a single upload at 10 MiB (mirrors the worker cap).
const maxImageBytes = 10 << 20

// maxImagesPerRequest keeps one multipart request from queueing an
// unbounded amount of work (and temp files).
const maxImagesPerRequest = 20

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
// @Description Ships {"data": [rows], "meta": {page, limit, offset, total, total_pages, has_next, has_prev}}.
// @Tags products
// @Produce json
// @Param q query string false "Full-text search over name, description and slug (word prefixes; substring fallback)"
// @Param ids query []string false "Restrict to product ids (comma separated or repeated, max MAX_BULK_ITEMS)"
// @Param category_id query string false "Filter by category UUID"
// @Param category_slug query string false "Filter by category slug"
// @Param min_price_cents query int false "Minimum price in USD cents"
// @Param max_price_cents query int false "Maximum price in USD cents"
// @Param in_stock query bool false "Only products with sellable stock (false = out of stock only)"
// @Param size query string false "Variant size name, e.g. XS, M, XL"
// @Param min_rating query number false "Minimum average review rating (0-5)"
// @Param created_after query string false "Created at or after (RFC3339 or YYYY-MM-DD)"
// @Param created_before query string false "Created at or before (RFC3339 or YYYY-MM-DD)"
// @Param active_only query bool false "Hide inactive (default true)"
// @Param sort query string false "name_asc|name_desc|price_asc|price_desc|newest|oldest|rating_asc|rating_desc|relevance (default relevance when q is set)"
// @Param page query int false "1-based page (wins over offset)"
// @Param limit query int false "Page size (default 12, max 100)"
// @Param offset query int false "Page offset"
// @Success 200 {object} public.ProductListResponse
// @Failure 400 {object} response.ErrorResponse
// @Router /products [get]
func (h *Handler) List(c *gin.Context) {
	var q dto.ListProductsQuery
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
	activeOnly := true
	if q.ActiveOnly != nil {
		activeOnly = *q.ActiveOnly
	}
	var ids []string
	if raw := c.QueryArray("ids"); len(raw) > 0 {
		parsed, err := bulk.ParseIDs(raw, bulk.MaxItems())
		if err != nil {
			response.Error(c, err)
			return
		}
		ids = parsed
	}
	page, err := h.svc.List(c.Request.Context(), public.ListFilter{
		IDs: ids, Query: q.Q, CategoryID: q.CategoryID, CategorySlug: q.CategorySlug,
		MinPriceCents: q.MinPriceCents, MaxPriceCents: q.MaxPriceCents,
		InStock: q.InStock, Size: q.Size, MinRating: q.MinRating,
		CreatedAfter: after, CreatedBefore: before, ActiveOnly: activeOnly,
		Sort: q.Sort, Page: q.Page, Limit: q.Limit, Offset: q.Offset,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OKMeta(c, page.Items, page.Meta)
}

// parseDateParam accepts RFC3339 timestamps and plain calendar dates.
// A date-only created_before expands to the end of that day, so
// "created_before=2026-10-28" includes the whole 28th.
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

// BulkCreate godoc
// @Summary Create products in bulk (admin)
// @Description Per-item results in a 200 envelope. Duplicate generated slugs
// @Description reject the whole request; a DB slug collision is per item.
// @Description dryRun validates and projects the rows without writing.
// @Tags admin
// @Accept json
// @Produce json
// @Param Idempotency-Key header string false "Replay-safe request key (24h)"
// @Param body body dto.BulkCreateProductsRequest true "Products"
// @Success 200 {object} dto.BulkCreateProductsResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /admin/products/bulk [post]
func (h *Handler) BulkCreate(c *gin.Context) {
	var req dto.BulkCreateProductsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.Validation("items is required"))
		return
	}
	items := make([]public.CreateProductInput, 0, len(req.Items))
	for _, in := range req.Items {
		items = append(items, public.CreateProductInput{
			CategoryID: in.CategoryID, Name: in.Name, Slug: in.Slug,
			Description: in.Description, PriceCents: in.PriceCents,
			Currency: in.Currency, ImageURL: in.ImageURL,
		})
	}
	started := time.Now()
	results, err := h.svc.BulkCreate(c.Request.Context(), items, req.Atomic, req.DryRun)
	if err != nil {
		response.Error(c, err)
		return
	}
	summary := bulk.Summary(results)
	bulk.Log(c, "products.bulk_create", started, summary)
	response.OK(c, dto.BulkCreateProductsResponse{Results: results, Summary: summary})
}

// BulkUpdate godoc
// @Summary Update products in bulk (admin)
// @Description One partial patch per product; slug stays immutable. An empty
// @Description patch is a per-item validation error. Duplicate ids reject the
// @Description whole request; dryRun projects without writing.
// @Tags admin
// @Accept json
// @Produce json
// @Param Idempotency-Key header string false "Replay-safe request key (24h)"
// @Param body body dto.BulkUpdateProductsRequest true "Patches"
// @Success 200 {object} dto.BulkUpdateProductsResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /admin/products/bulk [patch]
func (h *Handler) BulkUpdate(c *gin.Context) {
	var req dto.BulkUpdateProductsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.Validation("updates is required"))
		return
	}
	items := make([]public.BulkUpdateItem, 0, len(req.Updates))
	for _, u := range req.Updates {
		items = append(items, public.BulkUpdateItem{
			ID: u.ID,
			Patch: public.UpdateProductInput{
				CategoryID: u.Patch.CategoryID, Name: u.Patch.Name,
				Description: u.Patch.Description, PriceCents: u.Patch.PriceCents,
				Currency: u.Patch.Currency, IsActive: u.Patch.IsActive,
				ClearCategory: u.Patch.ClearCategory,
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
	bulk.Log(c, "products.bulk_update", started, summary)
	response.OK(c, dto.BulkUpdateProductsResponse{Results: results, Summary: summary})
}

// BulkDelete godoc
// @Summary Delete products in bulk (admin)
// @Description Max 25 ids. Products referenced by order history are blocked
// @Description per item (CONFLICT); variants, gallery, inventory, carts and
// @Description wishlists cascade. dryRun reports wouldDelete and blockers
// @Description without writing.
// @Tags admin
// @Accept json
// @Produce json
// @Param Idempotency-Key header string false "Replay-safe request key (24h)"
// @Param body body dto.BulkDeleteProductsRequest true "Ids"
// @Success 200 {object} dto.BulkDeleteProductsResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /admin/products/bulk-delete [post]
func (h *Handler) BulkDelete(c *gin.Context) {
	var req dto.BulkDeleteProductsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.Validation("ids is required"))
		return
	}
	started := time.Now()
	results, err := h.svc.BulkDelete(c.Request.Context(), req.IDs, req.Atomic, req.DryRun)
	if err != nil {
		response.Error(c, err)
		return
	}
	summary := bulk.Summary(results)
	bulk.Log(c, "products.bulk_delete", started, summary)
	response.OK(c, dto.BulkDeleteProductsResponse{Results: results, Summary: summary})
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

// ListImages godoc
// @Summary List a product's gallery images (admin)
// @Description Includes rows whose upload is still queued (empty url).
// @Tags admin
// @Produce json
// @Param id path string true "Product ID"
// @Success 200 {object} public.ImageListResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /admin/products/{id}/images [get]
func (h *Handler) ListImages(c *gin.Context) {
	images, err := h.svc.ListImages(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, public.ImageListResponse{Images: images})
}

// UploadImages godoc
// @Summary Upload product gallery images (admin)
// @Description Multipart with repeated files (field "files"; "file" is also accepted). Each file becomes one River job, so the response returns before Cloudinary finishes. The gallery fills in as uploads complete; when the product has no primary image, the first ready image is promoted to image_url automatically.
// @Tags admin
// @Accept multipart/form-data
// @Produce json
// @Param id path string true "Product ID"
// @Param files formData file true "Images (max 10 MiB each, up to 20 per request)"
// @Success 202 {object} dto.UploadImagesResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /admin/products/{id}/images [post]
func (h *Handler) UploadImages(c *gin.Context) {
	ctx := c.Request.Context()
	productID := c.Param("id")
	if _, err := h.svc.GetByID(ctx, productID); err != nil {
		response.Error(c, err)
		return
	}

	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		response.Error(c, apperr.Validation("multipart form with image files is required"))
		return
	}
	form := c.Request.MultipartForm
	if form == nil {
		response.Error(c, apperr.Validation("multipart form with image files is required"))
		return
	}
	defer form.RemoveAll()

	headers := form.File["files"]
	if len(headers) == 0 {
		headers = form.File["file"]
	}
	if len(headers) == 0 {
		response.Error(c, apperr.Validation("at least one image file is required"))
		return
	}
	if len(headers) > maxImagesPerRequest {
		response.Error(c, apperr.Validation("at most 20 images per request"))
		return
	}

	queued, failed := 0, 0
	var firstErr error
	for _, header := range headers {
		if header.Size > maxImageBytes {
			failed++
			if firstErr == nil {
				firstErr = apperr.Validation("image exceeds 10 MiB")
			}
			continue
		}

		img, err := h.svc.AddImage(ctx, productID)
		if err != nil {
			// Product vanished or the DB failed: stop queueing more, but
			// keep the jobs already accepted.
			if firstErr == nil {
				firstErr = err
			}
			break
		}

		tmpPath, err := saveUpload(header)
		if err != nil {
			_ = h.svc.DeleteImage(ctx, productID, img.ID)
			failed++
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		if err := h.media.Enqueue(ctx, mediapublic.EnqueueInput{
			OwnerType: mediapublic.OwnerProductImage,
			OwnerID:   img.ID,
			Filename:  header.Filename,
			Folder:    "malawi-store/products",
			TempPath:  tmpPath,
		}); err != nil {
			os.Remove(tmpPath)
			_ = h.svc.DeleteImage(ctx, productID, img.ID)
			failed++
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		queued++
	}

	if queued == 0 {
		if firstErr == nil {
			firstErr = apperr.Validation("no images could be queued")
		}
		response.Error(c, firstErr)
		return
	}
	response.Accepted(c, dto.UploadImagesResponse{
		Status: "queued", Queued: queued, Failed: failed,
	})
}

// saveUpload copies one multipart file into a temp file the River worker
// owns (and deletes after upload).
func saveUpload(header *multipart.FileHeader) (string, error) {
	src, err := header.Open()
	if err != nil {
		return "", apperr.Internal(err)
	}
	defer src.Close()

	tmp, err := os.CreateTemp("", "prodimg-*"+filepath.Ext(header.Filename))
	if err != nil {
		return "", apperr.Internal(err)
	}
	tmpPath := tmp.Name()
	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return "", apperr.Internal(err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return "", apperr.Internal(err)
	}
	return tmpPath, nil
}

// DeleteImage godoc
// @Summary Delete a product gallery image (admin)
// @Description If the row was the product's primary image, the next ready image is promoted; with no images left, image_url is cleared.
// @Tags admin
// @Produce json
// @Param id path string true "Product ID"
// @Param imageId path string true "Image ID"
// @Success 204
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /admin/products/{id}/images/{imageId} [delete]
func (h *Handler) DeleteImage(c *gin.Context) {
	if err := h.svc.DeleteImage(c.Request.Context(), c.Param("id"), c.Param("imageId")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

// SetPrimaryImage godoc
// @Summary Set a product's primary image (admin)
// @Description Points products.image_url (the storefront primary) at one gallery row. Pending uploads are rejected.
// @Tags admin
// @Produce json
// @Param id path string true "Product ID"
// @Param imageId path string true "Image ID"
// @Success 200 {object} public.Product
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /admin/products/{id}/images/{imageId}/primary [put]
func (h *Handler) SetPrimaryImage(c *gin.Context) {
	p, err := h.svc.SetPrimaryImage(c.Request.Context(), c.Param("id"), c.Param("imageId"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, p)
}
