package model

import "time"

// Verification is an email_verifications row. Module-private.
type Verification struct {
	ID        string
	UserID    string
	CodeHash  string
	ExpiresAt time.Time
	Attempts  int
}

// PasswordReset is a password_resets row. Module-private.
type PasswordReset struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
}

// RefreshToken is a refresh_tokens row. Module-private.
type RefreshToken struct {
	ID         string
	UserID     string
	TokenHash  string
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	ReplacedBy *string
}
