package dto

// ListOrdersQuery binds the caller's order-history filters. Dates accept
// RFC3339 or YYYY-MM-DD; page wins over offset when both are sent.
type ListOrdersQuery struct {
	Status        string `form:"status"`
	Currency      string `form:"currency"`
	CreatedAfter  string `form:"created_after"`
	CreatedBefore string `form:"created_before"`
	MinTotalCents *int64 `form:"min_total_cents"`
	MaxTotalCents *int64 `form:"max_total_cents"`
	Sort          string `form:"sort"`
	Page          int    `form:"page"`
	Limit         int    `form:"limit"`
	Offset        int    `form:"offset"`
}
