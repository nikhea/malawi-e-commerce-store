package model

import "time"

// Review is the reviews table row. Module-private. Author is the display
// name snapshot at write time (users.name may change; the words stay as
// written). User rows cascade, so a deleted author's review goes with
// them — the snapshot only protects against renames, not deletion.
type Review struct {
	ID        string
	UserID    string
	Author    string
	ProductID string
	Rating    int
	Title     string
	Body      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ListFilter scopes a product's paged review list. The aggregate
// (average/count) always covers every review, not just the page.
type ListFilter struct {
	Query  string
	Rating *int
	Sort   string
	Limit  int
	Offset int
}
