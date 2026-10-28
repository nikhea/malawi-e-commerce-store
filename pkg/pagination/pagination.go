// Package pagination holds the catalog-wide paging contract: page size
// limits and the metadata every list response ships next to its rows.
// Domain-free so products, categories and future catalog modules share
// one definition.
package pagination

// DefaultLimit is the page size when the caller omits one.
const DefaultLimit = 20

// MaxLimit caps a page so a single request cannot pull the whole catalog.
const MaxLimit = 100

// Meta describes the page a list response contains. Offsets stay in the
// payload because the API accepts both `page` and `offset`; clients can
// continue from either.
type Meta struct {
	Page       int   `json:"page" example:"1"`
	Limit      int   `json:"limit" example:"20"`
	Offset     int   `json:"offset" example:"0"`
	Total      int64 `json:"total" example:"50"`
	TotalPages int   `json:"total_pages" example:"3"`
	HasNext    bool  `json:"has_next" example:"true"`
	HasPrev    bool  `json:"has_prev" example:"false"`
}

// Normalize resolves the three paging inputs into a bounded limit and
// offset. A positive page wins over offset (page 2 + limit 20 = offset
// 20), which keeps the two styles from disagreeing. Negativity is
// validated by the caller so it can return a 400.
func Normalize(page, limit, offset int) (int, int) {
	if limit <= 0 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}
	if page > 0 {
		offset = (page - 1) * limit
	}
	return limit, offset
}

// NewMeta builds the response metadata for one page. limit must already
// be normalized.
func NewMeta(total int64, limit, offset int) Meta {
	page := 1
	if limit > 0 {
		page = offset/limit + 1
	}
	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(limit) - 1) / int64(limit))
	}
	return Meta{
		Page:       page,
		Limit:      limit,
		Offset:     offset,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    int64(offset+limit) < total,
		HasPrev:    offset > 0,
	}
}
