package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/internal/users/dto"
	"github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	"github.com/nikhea/malawi-e-commerce-store/pkg/middleware"
	"github.com/nikhea/malawi-e-commerce-store/pkg/response"
)

// Handler is thin by design: bind input, call the service, write the
// envelope. No business logic, no DB access.
type Handler struct {
	svc public.Service
}

func New(svc public.Service) *Handler {
	return &Handler{svc: svc}
}

// GetByID godoc
// @Summary Get user by ID
// @Description Returns one account wrapped in the standard {"data"} envelope.
// @Tags users
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} public.User
// @Failure 404 {object} response.ErrorResponse
// @Router /users/{id} [get]
func (h *Handler) GetByID(c *gin.Context) {
	u, err := h.svc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, u)
}

// Me godoc
// @Summary Get the caller's own account
// @Description Reads identity from the JWT (no id parameter, can't spoof others).
// @Tags users
// @Produce json
// @Success 200 {object} public.User
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /users/me [get]
func (h *Handler) Me(c *gin.Context) {
	id := middleware.UserIDOf(c)
	if id == "" {
		response.Error(c, apperr.Unauthorized("authentication required"))
		return
	}
	u, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, u)
}

// SetRole godoc
// @Summary Change a user's role (admin)
// @Description Single-storefront admin op. Requires the admin role.
// @Tags admin
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param body body dto.SetRoleRequest true "New role"
// @Success 200 {object} public.User
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /admin/users/{id}/role [patch]
func (h *Handler) SetRole(c *gin.Context) {
	var req dto.SetRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.Validation("role is required"))
		return
	}
	u, err := h.svc.SetRole(c.Request.Context(), c.Param("id"), req.Role)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, u)
}
