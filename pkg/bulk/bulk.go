// Package bulk is the domain-free kernel shared by every bulk endpoint:
// batch caps, duplicate detection, query-id parsing, result construction,
// and the one-line request log.
package bulk

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	"github.com/nikhea/malawi-e-commerce-store/pkg/middleware"
)

// DefaultMaxItems is the global batch cap when MAX_BULK_ITEMS is unset.
const DefaultMaxItems = 50

var maxItems atomic.Int64

func init() { maxItems.Store(DefaultMaxItems) }

// SetMaxItems applies the configured cap. Non-positive input is ignored so
// a bad env value cannot disable the guard.
func SetMaxItems(n int) {
	if n > 0 {
		maxItems.Store(int64(n))
	}
}

// MaxItems returns the active batch cap.
func MaxItems() int { return int(maxItems.Load()) }

// CheckCount rejects empty batches and batches over the cap. limit <= 0
// means "use the configured MAX_BULK_ITEMS".
func CheckCount(n, limit int) error {
	if n == 0 {
		return apperr.Validation("at least one item is required")
	}
	if limit <= 0 {
		limit = MaxItems()
	}
	if n > limit {
		return apperr.BulkLimitExceeded(fmt.Sprintf("at most %d items per request", limit))
	}
	return nil
}

// ParseIDs accepts repeated query values and comma-separated values in one
// value (`?ids=a&ids=b,c`), trims blanks, rejects duplicates, and caps the
// result. Used by GET /products?ids= and inventory's variantIds filter.
func ParseIDs(values []string, limit int) ([]string, error) {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		for _, part := range strings.Split(raw, ",") {
			id := strings.TrimSpace(part)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				return nil, apperr.Duplicate("duplicate id: " + id)
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	if len(out) == 0 {
		return nil, apperr.Validation("at least one id is required")
	}
	if limit <= 0 {
		limit = MaxItems()
	}
	if len(out) > limit {
		return nil, apperr.BulkLimitExceeded(fmt.Sprintf("at most %d ids per request", limit))
	}
	return out, nil
}

// FindDuplicate returns the first repeated key, if any. Bulk requests are
// rejected up front on duplicates so a single key can never mean two
// different writes.
func FindDuplicate(keys []string) (string, bool) {
	seen := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		if _, ok := seen[k]; ok {
			return k, true
		}
		seen[k] = struct{}{}
	}
	return "", false
}

// LineKey builds a stable duplicate key from a product/variant pair.
func LineKey(productID string, variantID *string) string {
	if variantID == nil {
		return productID + "|"
	}
	return productID + "|" + *variantID
}

// OK builds a successful item.
func OK[T any](index int, id string, data T) BulkResult[T] {
	return BulkResult[T]{Index: index, ID: id, Status: "ok", Data: &data}
}

// Failed builds an error item. Foreign errors become INTERNAL with the
// generic message so internals never leak.
func Failed[T any](index int, id string, err error) BulkResult[T] {
	e := BulkItemError{Code: apperr.CodeOf(err), Message: "internal server error"}
	var appErr *apperr.Error
	if errors.As(err, &appErr) && appErr.Message != "" && appErr.Code != apperr.CodeInternal {
		e.Message = appErr.Message
	}
	return BulkResult[T]{Index: index, ID: id, Status: "error", Error: &e}
}

// Rollback marks every item rolled_back. Only the first failing item
// carries the real error; the rest stay silent per the bulk contract.
func Rollback[T any](results []BulkResult[T], failIndex int, err error) []BulkResult[T] {
	for i := range results {
		results[i].Status = "rolled_back"
		results[i].Data = nil
		results[i].Error = nil
	}
	if failIndex >= 0 && failIndex < len(results) && err != nil {
		results[failIndex].Error = Failed[T](failIndex, results[failIndex].ID, err).Error
	}
	return results
}

// Summary counts a result set. Anything not "ok" (error or rolled_back)
// counts as failed.
func Summary[T any](results []BulkResult[T]) BulkSummary {
	s := BulkSummary{Total: len(results)}
	for _, r := range results {
		if r.Status == "ok" {
			s.Succeeded++
			continue
		}
		s.Failed++
	}
	return s
}

// Log writes the single audit line every bulk request emits. There is no
// per-item admin audit trail in this codebase today.
func Log(c *gin.Context, endpoint string, started time.Time, s BulkSummary) {
	log.Printf("bulk user=%s endpoint=%s total=%d succeeded=%d failed=%d duration=%s request_id=%s",
		middleware.UserIDOf(c), endpoint, s.Total, s.Succeeded, s.Failed,
		time.Since(started), middleware.RequestIDOf(c))
}
