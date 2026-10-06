package dto

// RegisterRequest is the POST /auth/register body. Exported so swag can
// render it in generated API docs.
type RegisterRequest struct {
	Email    string `json:"email" binding:"required" example:"shop@malawi.mw"`
	Name     string `json:"name" example:"Aisha Banda"`
	Password string `json:"password" binding:"required" example:"hunter2-secure"`
}

// LoginRequest is the POST /auth/login body. Exported for swag.
type LoginRequest struct {
	Email    string `json:"email" binding:"required" example:"shop@malawi.mw"`
	Password string `json:"password" binding:"required" example:"hunter2-secure"`
}

// VerifyEmailRequest is the POST /auth/verify-email body (OTP code).
type VerifyEmailRequest struct {
	Email string `json:"email" binding:"required" example:"shop@malawi.mw"`
	Code  string `json:"code" binding:"required" example:"482914"`
}

// PasswordForgotRequest is the POST /auth/password/forgot body.
type PasswordForgotRequest struct {
	Email string `json:"email" binding:"required" example:"shop@malawi.mw"`
}

// PasswordResetRequest is the POST /auth/password/reset body.
type PasswordResetRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required" example:"hunter2-new"`
}

// RefreshRequest is the POST /auth/refresh body.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}
