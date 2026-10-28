package handler

import (
	"strings"
	"time"

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

// List godoc
// @Summary List users (admin)
// @Description Admin user directory: search, role/verification filters and paging.
// @Tags admin
// @Produce json
// @Param q query string false "Search email or name"
// @Param role query string false "admin|customer"
// @Param email_verified query bool false "Filter by verification state"
// @Param created_after query string false "Created at or after (RFC3339 or YYYY-MM-DD)"
// @Param created_before query string false "Created at or before (RFC3339 or YYYY-MM-DD)"
// @Param sort query string false "newest|oldest|name_asc|name_desc|email_asc|email_desc"
// @Param page query int false "1-based page (wins over offset)"
// @Param limit query int false "Page size (default 20, max 100)"
// @Param offset query int false "Page offset"
// @Success 200 {object} public.UserListResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /admin/users [get]
func (h *Handler) List(c *gin.Context) {
	var q dto.ListUsersQuery
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
	page, err := h.svc.List(c.Request.Context(), public.ListFilter{
		Query: q.Q, Role: public.Role(strings.ToLower(strings.TrimSpace(q.Role))),
		EmailVerified: q.EmailVerified, CreatedAfter: after, CreatedBefore: before,
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
