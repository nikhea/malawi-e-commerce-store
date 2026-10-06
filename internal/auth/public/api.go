// Package public is the auth module's external contract. It is the ONLY
// auth package other modules and cmd/* may import. Service, utils,
// handler stay module-private.
package public

import (
	"context"
	"time"

	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
)

// Claims is the verified JWT payload. The middleware trusts these WITHOUT
// a DB hit, so they stay minimal: identity + role + expiry.
type Claims struct {
	UserID string
	Email  string
	Role   userspublic.Role
	Expiry time.Time
}

// TokenPair is what register/login/refresh hand the client. Account
// fields are flattened scalars (not nested module types) so generated
// API docs resolve without cross-package ambiguity.
type TokenPair struct {
	Token          string    `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	ExpiresAt      time.Time `json:"expires_at"`
	RefreshToken   string    `json:"refresh_token" example:"w8X2..."`
	RefreshExpires time.Time `json:"refresh_expires_at"`
	UserID         string    `json:"user_id" example:"3fa85f64-5717-4562-b3fc-2c963f66afa6"`
	Email          string    `json:"email" example:"shop@malawi.mw"`
	Name           string    `json:"name" example:"Aisha Banda"`
	Role           string    `json:"role" example:"customer"`
}

// RegisterInput carries a signup. Password is RAW here — hashed inside
// the service, never stored raw anywhere.
type RegisterInput struct {
	Email    string
	Name     string
	Password string
}

// Service is the auth capability surface. Every method takes ctx first.
type Service interface {
	Register(ctx context.Context, in RegisterInput) (TokenPair, error)
	Login(ctx context.Context, email, password string) (TokenPair, error)
	// Parse verifies a token for the JWT middleware. Exposed on the
	// contract (not utils) so callers depend on auth's promise, and so
	// the middleware can take this interface instead of the service.
	Parse(token string) (Claims, error)
	// RequestVerification (re)sends the OTP email. Idempotent: always
	// succeeds for existing accounts (unknown emails also succeed —
	// no enumeration).
	RequestVerification(ctx context.Context, email string) error
	// VerifyEmail checks the OTP and flips the verified flag.
	VerifyEmail(ctx context.Context, email, code string) error
	// RequestPasswordReset emails a reset link. Always succeeds (no
	// enumeration); unknown emails just don't send.
	RequestPasswordReset(ctx context.Context, email string) error
	// ResetPassword consumes a reset token and sets the new password,
	// revoking all refresh tokens (stolen-session lockout).
	ResetPassword(ctx context.Context, token, newPassword string) error
	// Refresh rotates a refresh token into a fresh pair. Reuse of a
	// consumed token revokes the whole chain (theft detection).
	Refresh(ctx context.Context, refreshToken string) (TokenPair, error)
	// Logout revokes one refresh token. Unknown tokens succeed silently.
	Logout(ctx context.Context, refreshToken string) error
}
