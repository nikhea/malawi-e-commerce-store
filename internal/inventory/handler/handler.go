package handler

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/internal/inventory/dto"
	"github.com/nikhea/malawi-e-commerce-store/internal/inventory/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	"github.com/nikhea/malawi-e-commerce-store/pkg/bulk"
	"github.com/nikhea/malawi-e-commerce-store/pkg/response"
)

// Handler is thin: bind input, call the service, write the envelope.
// Reserve/release/confirm are admin HTTP here for ops; orders and
// payments call the Service directly (no HTTP between modules).
type Handler struct {
	svc public.Service
}

func New(svc public.Service) *Handler {
	return &Handler{svc: svc}
}

// SetStock godoc
// @Summary Set on-hand stock (admin)
// @Tags admin
// @Accept json
// @Produce json
// @Param body body dto.SetStockRequest true "Stock"
// @Success 200 {object} public.Stock
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /admin/inventory/stock [post]
func (h *Handler) SetStock(c *gin.Context) {
	var req dto.SetStockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.Validation("product_id and qty are required"))
		return
	}
	stock, err := h.svc.SetStock(c.Request.Context(), req.ProductID, req.VariantID, req.Qty)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, stock)
}

// GetStock godoc
// @Summary Read availability (admin)
// @Description With variantIds, returns one row per variant (comma lists or
// @Description repeated query params, cap MAX_BULK_ITEMS) plus the ids that do
// @Description not resolve to a variant. Without variantIds the single
// @Description product_id read is unchanged.
// @Tags admin
// @Produce json
// @Param product_id query string false "Product ID (single read)"
// @Param variant_id query string false "Variant ID (single read)"
// @Param variantIds query []string false "Variant IDs (comma separated or repeated)"
// @Success 200 {object} public.Stock "Single stock row when variantIds is absent"
// @Success 200 {object} dto.BulkStockResponse "Variant batch read"
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /admin/inventory/stock [get]
func (h *Handler) GetStock(c *gin.Context) {
	if hasVariantIDs(c) {
		ids, err := bulk.ParseIDs(c.QueryArray("variantIds"), bulk.MaxItems())
		if err != nil {
			response.Error(c, err)
			return
		}
		stock, notFound, err := h.svc.GetStockByVariants(c.Request.Context(), ids)
		if err != nil {
			response.Error(c, err)
			return
		}
		response.OK(c, dto.BulkStockResponse{Stock: stock, NotFound: notFound})
		return
	}
	productID := c.Query("product_id")
	if productID == "" {
		response.Error(c, apperr.Validation("product_id is required"))
		return
	}
	var variantID *string
	if v := c.Query("variant_id"); v != "" {
		variantID = &v
	}
	stock, err := h.svc.GetStock(c.Request.Context(), productID, variantID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, stock)
}

func hasVariantIDs(c *gin.Context) bool {
	for _, v := range c.QueryArray("variantIds") {
		if strings.TrimSpace(v) != "" {
			return true
		}
	}
	return false
}

// BulkSetStock godoc
// @Summary Set on-hand stock for many SKUs (admin)
// @Description Per-item results; HTTP 200 for a processed batch. With
// @Description atomic=true every line commits or the whole batch rolls back.
// @Tags admin
// @Accept json
// @Produce json
// @Param Idempotency-Key header string false "Replay-safe request key (24h)"
// @Param body body dto.BulkSetStockRequest true "Stock lines"
// @Success 200 {object} dto.BulkSetStockResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /admin/inventory/stock/bulk [post]
func (h *Handler) BulkSetStock(c *gin.Context) {
	var req dto.BulkSetStockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.Validation("items is required"))
		return
	}
	items := make([]public.BulkStockItem, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, public.BulkStockItem{
			ProductID: it.ProductID, VariantID: it.VariantID, Qty: it.Qty,
		})
	}
	started := time.Now()
	results, err := h.svc.BulkSetStock(c.Request.Context(), items, req.Atomic)
	if err != nil {
		response.Error(c, err)
		return
	}
	summary := bulk.Summary(results)
	bulk.Log(c, "inventory.stock.bulk", started, summary)
	response.OK(c, dto.BulkSetStockResponse{Results: results, Summary: summary})
}

// Reserve godoc
// @Summary Reserve stock for an order (admin)
// @Description Holds lines atomically; short lines fail the whole order.
// @Tags admin
// @Accept json
// @Produce json
// @Param body body dto.ReserveRequest true "Reservation"
// @Success 200 {object} map[string]string
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /admin/inventory/reserve [post]
func (h *Handler) Reserve(c *gin.Context) {
	var req dto.ReserveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.Validation("order_ref and lines are required"))
		return
	}
	lines := make([]public.ReserveLine, 0, len(req.Lines))
	for _, l := range req.Lines {
		lines = append(lines, public.ReserveLine{ProductID: l.ProductID, VariantID: l.VariantID, Qty: l.Qty})
	}
	if err := h.svc.Reserve(c.Request.Context(), req.OrderRef, lines, time.Duration(req.TTLSeconds)*time.Second); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"status": "reserved"})
}

// Release godoc
// @Summary Release an order's holds (admin)
// @Description Idempotent: releasing twice succeeds.
// @Tags admin
// @Accept json
// @Produce json
// @Param body body dto.OrderRefRequest true "Order"
// @Success 200 {object} map[string]string
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /admin/inventory/release [post]
func (h *Handler) Release(c *gin.Context) {
	var req dto.OrderRefRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.Validation("order_ref is required"))
		return
	}
	if err := h.svc.ReleaseByOrder(c.Request.Context(), req.OrderRef); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"status": "released"})
}

// Confirm godoc
// @Summary Confirm an order's holds into sales (admin)
// @Description Idempotent: confirming twice succeeds.
// @Tags admin
// @Accept json
// @Produce json
// @Param body body dto.OrderRefRequest true "Order"
// @Success 200 {object} map[string]string
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /admin/inventory/confirm [post]
func (h *Handler) Confirm(c *gin.Context) {
	var req dto.OrderRefRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.Validation("order_ref is required"))
		return
	}
	if err := h.svc.ConfirmByOrder(c.Request.Context(), req.OrderRef); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"status": "confirmed"})
}
