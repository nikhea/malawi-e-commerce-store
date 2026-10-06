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

// RequestVerification godoc
// @Summary Resend the verification OTP
// @Description Always succeeds (unknown emails too — no enumeration).
// @Tags auth
// @Accept json
// @Produce json
// @Param body body dto.PasswordForgotRequest true "Email"
// @Success 200 {object} map[string]string
// @Router /auth/verify-email/request [post]
func (h *Handler) RequestVerification(c *gin.Context) {
	var req dto.PasswordForgotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.Validation("email is required"))
		return
	}
	if err := h.svc.RequestVerification(c.Request.Context(), req.Email); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"status": "sent"})
}

// VerifyEmail godoc
// @Summary Verify email with OTP code
// @Tags auth
// @Accept json
// @Produce json
// @Param body body dto.VerifyEmailRequest true "Email + code"
// @Success 200 {object} map[string]string
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Router /auth/verify-email [post]
func (h *Handler) VerifyEmail(c *gin.Context) {
	var req dto.VerifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.Validation("email and code are required"))
		return
	}
	if err := h.svc.VerifyEmail(c.Request.Context(), req.Email, req.Code); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"status": "verified"})
}

// RequestPasswordReset godoc
// @Summary Request a password reset email
// @Description Always succeeds (unknown emails too — no enumeration).
// @Tags auth
// @Accept json
// @Produce json
// @Param body body dto.PasswordForgotRequest true "Email"
// @Success 200 {object} map[string]string
// @Router /auth/password/forgot [post]
func (h *Handler) RequestPasswordReset(c *gin.Context) {
	var req dto.PasswordForgotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.Validation("email is required"))
		return
	}
	if err := h.svc.RequestPasswordReset(c.Request.Context(), req.Email); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"status": "sent"})
}

// ResetPassword godoc
// @Summary Reset password with token
// @Description Consumes the token, sets the password, revokes sessions.
// @Tags auth
// @Accept json
// @Produce json
// @Param body body dto.PasswordResetRequest true "Token + new password"
// @Success 200 {object} map[string]string
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Router /auth/password/reset [post]
func (h *Handler) ResetPassword(c *gin.Context) {
	var req dto.PasswordResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.Validation("token and new password are required"))
		return
	}
	if err := h.svc.ResetPassword(c.Request.Context(), req.Token, req.NewPassword); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"status": "reset"})
}

// Refresh godoc
// @Summary Rotate into a fresh token pair
// @Description Reusing a consumed token kills the whole session chain.
// @Tags auth
// @Accept json
// @Produce json
// @Param body body dto.RefreshRequest true "Refresh token"
// @Success 200 {object} public.TokenPair
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Router /auth/refresh [post]
func (h *Handler) Refresh(c *gin.Context) {
	var req dto.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.Validation("refresh token is required"))
		return
	}
	pair, err := h.svc.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, pair)
}

// Logout godoc
// @Summary Revoke a refresh token
// @Tags auth
// @Accept json
// @Produce json
// @Param body body dto.RefreshRequest true "Refresh token"
// @Success 204
// @Router /auth/logout [post]
func (h *Handler) Logout(c *gin.Context) {
	var req dto.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperr.Validation("refresh token is required"))
		return
	}
	if err := h.svc.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}
