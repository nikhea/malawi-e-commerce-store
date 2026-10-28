package model

import "time"

// ListFilter is the repository-level taxonomy query. Values are already
// normalized and validated by the service layer.
type ListFilter struct {
	Query       string
	ParentID    *string
	RootOnly    bool
	HasProducts *bool
	Sort        string
	Limit       int
	Offset      int
}

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
