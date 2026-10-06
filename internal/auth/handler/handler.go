package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/internal/auth/dto"
	"github.com/nikhea/malawi-e-commerce-store/internal/auth/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	"github.com/nikhea/malawi-e-commerce-store/pkg/response"
)

// Handler is thin: bind input, call the service, write the envelope.
type Handler struct {
	svc public.Service
}

func New(svc public.Service) *Handler {
	return &Handler{svc: svc}
}

// Register godoc
// @Summary Register a new account
// @Description Creates the account (customer by default, admin for ADMIN_EMAILS) and returns a JWT.
// @Tags auth
// @Accept json
// @Produce json
// @Param body body dto.RegisterRequest true "Signup"
// @Success 201 {object} public.TokenPair
// @Failure 400 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Router /auth/register [post]
func (h *Handler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.Validation("email, name and password are required"))
		return
	}
	pair, err := h.svc.Register(c.Request.Context(), public.RegisterInput{
		Email: req.Email, Name: req.Name, Password: req.Password,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, pair)
}

// Login godoc
// @Summary Log in
// @Description Verifies credentials and returns a JWT. Errors stay vague on purpose.
// @Tags auth
// @Accept json
// @Produce json
// @Param body body dto.LoginRequest true "Credentials"
// @Success 200 {object} public.TokenPair
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Router /auth/login [post]
func (h *Handler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.Validation("email and password are required"))
		return
	}
	pair, err := h.svc.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, pair)
}
