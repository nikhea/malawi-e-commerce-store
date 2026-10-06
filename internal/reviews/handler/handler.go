package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/internal/reviews/dto"
	"github.com/nikhea/malawi-e-commerce-store/internal/reviews/public"
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

// Summary godoc
// @Summary Product reviews + aggregate
// @Description Public page: average, count, newest-first reviews.
// @Tags products
// @Produce json
// @Param id path string true "Product ID"
// @Success 200 {object} public.Summary
// @Router /products/{id}/reviews [get]
func (h *Handler) Summary(c *gin.Context) {
	summary, err := h.svc.Summary(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, summary)
}

// Create godoc
// @Summary Review a product
// @Description One review per user per product — edit to change it.
// @Tags reviews
// @Accept json
// @Produce json
// @Param id path string true "Product ID"
// @Param body body dto.CreateReviewRequest true "Verdict"
// @Success 201 {object} public.Review
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /products/{id}/reviews [post]
func (h *Handler) Create(c *gin.Context) {
	var req dto.CreateReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.Validation("rating is required"))
		return
	}
	rev, err := h.svc.Create(c.Request.Context(), public.CreateReviewInput{
		UserID: middleware.UserIDOf(c), ProductID: c.Param("id"),
		Rating: req.Rating, Title: req.Title, Body: req.Body,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, rev)
}

// UpdateMine godoc
// @Summary Edit my review
// @Tags reviews
// @Accept json
// @Produce json
// @Param id path string true "Review ID"
// @Param body body dto.UpdateReviewRequest true "Fields"
// @Success 200 {object} public.Review
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /reviews/{id} [patch]
func (h *Handler) UpdateMine(c *gin.Context) {
	var req dto.UpdateReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.Validation("invalid body"))
		return
	}
	rev, err := h.svc.UpdateMine(c.Request.Context(), middleware.UserIDOf(c), c.Param("id"), public.UpdateReviewInput{
		Rating: req.Rating, Title: req.Title, Body: req.Body,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, rev)
}

// DeleteMine godoc
// @Summary Delete my review
// @Tags reviews
// @Produce json
// @Param id path string true "Review ID"
// @Success 204
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /reviews/{id} [delete]
func (h *Handler) DeleteMine(c *gin.Context) {
	if err := h.svc.DeleteMine(c.Request.Context(), middleware.UserIDOf(c), c.Param("id")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

// DeleteAny godoc
// @Summary Remove any review (admin)
// @Description Moderation: abusive or spam reviews.
// @Tags admin
// @Produce json
// @Param id path string true "Review ID"
// @Success 204
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /admin/reviews/{id} [delete]
func (h *Handler) DeleteAny(c *gin.Context) {
	if err := h.svc.DeleteAny(c.Request.Context(), c.Param("id")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}
