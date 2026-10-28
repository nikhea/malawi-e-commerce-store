package model

import "time"

// ListFilter is the repository-level user directory query. Values are
// already normalized and validated by the service layer.
type ListFilter struct {
	Query         string
	Role          string
	EmailVerified *bool
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	Sort          string
	Limit         int
	Offset        int
}

// User is the users table row. Module-private: other modules see
// public.User, never this struct (password_hash must not cross the
// module boundary).
type User struct {
	ID            string
	Email         string
	PasswordHash  string
	Name          string
	Role          string
	EmailVerified bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
