package model

import "time"

// User is the users table row. Module-private: other modules see
// public.User, never this struct (password_hash must not cross the
// module boundary).
type User struct {
	ID           string
	Email        string
	PasswordHash string
	Name         string
	Role         string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
