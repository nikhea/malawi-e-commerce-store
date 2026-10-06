package model

import "time"

// Category is the categories table row. Module-private.
type Category struct {
	ID            string
	Name          string
	Slug          string
	Description   string
	ParentID      *string
	ImageURL      string
	ImagePublicID string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
