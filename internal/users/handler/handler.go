package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
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

func (h *Handler) GetByID(c *gin.Context) {
	u, err := h.svc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, u)
}

type setRoleRequest struct {
	Role public.Role `json:"role" binding:"required"`
}

func (h *Handler) SetRole(c *gin.Context) {
	var req setRoleRequest
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
