// Package public is the users module's external contract. It is the ONLY
// users package other modules and cmd/* may import. Service, repository,
// handler, and model stay module-private.
package public

import (
	"context"
	"time"
)

// Role is the single-storefront access level. One store, no sellers:
// 'admin' manages catalog/orders/users, 'customer' shops.
type Role string

const (
	RoleAdmin    Role = "admin"
	RoleCustomer Role = "customer"
)

// User is the cross-module view of an account. It never carries the
// password hash — only Credentials does, and only to the auth module.
type User struct {
	ID            string    `json:"id" example:"3fa85f64-5717-4562-b3fc-2c963f66afa6"`
	Email         string    `json:"email" example:"shop@malawi.mw"`
	Name          string    `json:"name" example:"Aisha Banda"`
	Role          Role      `json:"role" example:"customer"`
	EmailVerified bool      `json:"email_verified"`
	CreatedAt     time.Time `json:"created_at"`
}

// CreateUserInput carries everything needed to register an account.
// PasswordHash is produced by the auth module (bcrypt); users only stores.
// Role zero value ("") means customer.
type CreateUserInput struct {
	Email        string
	Name         string
	PasswordHash string
	Role         Role
}

// Credentials is the login-verification view: the ONLY shape that carries
// the password hash across the module boundary, and only to the auth
// module. No JSON tags on purpose — this must never be serialized.
type Credentials struct {
	UserID       string
	Email        string
	Role         Role
	PasswordHash string
}

// Service is the users capability surface. Every method takes ctx first.
type Service interface {
	Create(ctx context.Context, in CreateUserInput) (User, error)
	GetByID(ctx context.Context, id string) (User, error)
	GetByEmail(ctx context.Context, email string) (User, error)
	// GetCredentials returns the login-verification view for auth's
	// password check. Same not-found semantics as GetByEmail.
	GetCredentials(ctx context.Context, email string) (Credentials, error)
	// SetRole changes an account's access level. Admin-only: enforced by
	// the RequireRole middleware on /admin routes, not by this package.
	SetRole(ctx context.Context, id string, role Role) (User, error)
	// SetEmailVerified flips the verification flag (auth's OTP flow).
	SetEmailVerified(ctx context.Context, id string, verified bool) (User, error)
	// SetPasswordHash replaces the stored hash (auth's reset flow).
	SetPasswordHash(ctx context.Context, id, hash string) (User, error)
}
