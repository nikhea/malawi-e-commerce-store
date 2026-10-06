package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/internal/wishlist/dto"
	"github.com/nikhea/malawi-e-commerce-store/internal/wishlist/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	"github.com/nikhea/malawi-e-commerce-store/pkg/middleware"
	"github.com/nikhea/malawi-e-commerce-store/pkg/response"
)

// Handler is thin: identity from the JWT, call the service.
type Handler struct {
	svc public.Service
}

func New(svc public.Service) *Handler {
	return &Handler{svc: svc}
}

// ListMine godoc
// @Summary List my wishlist
// @Tags wishlist
// @Produce json
// @Success 200 {array} public.Item
// @Failure 401 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /wishlist [get]
func (h *Handler) ListMine(c *gin.Context) {
	items, err := h.svc.ListMine(c.Request.Context(), middleware.UserIDOf(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, items)
}

// Add godoc
// @Summary Heart a product
// @Description Idempotent: re-hearting succeeds silently.
// @Tags wishlist
// @Accept json
// @Produce json
// @Param body body dto.AddWishlistRequest true "Product"
// @Success 201 {object} public.Item
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /wishlist [post]
func (h *Handler) Add(c *gin.Context) {
	var req dto.AddWishlistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.Validation("product_id is required"))
		return
	}
	item, err := h.svc.Add(c.Request.Context(), middleware.UserIDOf(c), req.ProductID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, item)
}

// Remove godoc
// @Summary Unheart a product
// @Tags wishlist
// @Produce json
// @Param id path string true "Product ID"
// @Success 200 {object} map[string]string
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /wishlist/{id} [delete]
func (h *Handler) Remove(c *gin.Context) {
	if err := h.svc.Remove(c.Request.Context(), middleware.UserIDOf(c), c.Param("id")); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"status": "removed"})
}
