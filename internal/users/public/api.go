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

// User is the cross-module view of an account. PasswordHash NEVER leaves
// this module — not in this struct, not in any method signature here.
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      Role      `json:"role"`
	CreatedAt time.Time `json:"created_at"`
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

// Service is the users capability surface. Every method takes ctx first.
type Service interface {
	Create(ctx context.Context, in CreateUserInput) (User, error)
	GetByID(ctx context.Context, id string) (User, error)
	GetByEmail(ctx context.Context, email string) (User, error)
	// SetRole changes an account's access level. Admin-only: enforced by
	// the RequireRole middleware on /admin routes, not by this package.
	SetRole(ctx context.Context, id string, role Role) (User, error)
}
