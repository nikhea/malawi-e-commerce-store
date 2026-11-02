// Package dto holds the shared wire shapes for bulk endpoints. Every bulk
// response keeps per-item results so one bad row never kills the batch.
//
// Swag (1.16.x) cannot parse a generic instantiation inside an annotation,
// so modules declare a named type based on BulkResponse[T] (for example
// `type BulkSetStockResponse bulkdto.BulkResponse[public.Stock]`). Swag
// resolves that named type correctly.
package dto

import "github.com/nikhea/malawi-e-commerce-store/pkg/apperr"

// BulkSummary is the batch outcome. Succeeded counts only "ok" items;
// failed covers error and rolled_back items.
type BulkSummary struct {
	Total     int `json:"total" example:"2"`
	Succeeded int `json:"succeeded" example:"1"`
	Failed    int `json:"failed" example:"1"`
}

// BulkItemError is one machine-readable per-item failure.
type BulkItemError struct {
	Code    apperr.Code `json:"code" example:"NOT_FOUND"`
	Message string      `json:"message" example:"product not found"`
}

// BulkResult is one item's outcome. Index is the request position so
// clients can line results up with their input even when some fail.
//
// The bulk-only flags are omitted when false: AlreadyExists/AlreadyRemoved
// make idempotent customer writes explicit, while WouldDelete/Blockers
// describe a dry run.
type BulkResult[T any] struct {
	Index          int            `json:"index" example:"0"`
	ID             string         `json:"id,omitempty"`
	Status         string         `json:"status" example:"ok"`
	Data           *T             `json:"data,omitempty"`
	Error          *BulkItemError `json:"error,omitempty"`
	AlreadyExists  bool           `json:"alreadyExists,omitempty"`
	AlreadyRemoved bool           `json:"alreadyRemoved,omitempty"`
	WouldDelete    bool           `json:"wouldDelete,omitempty"`
	Blockers       []string       `json:"blockers,omitempty"`
}

// BulkResponse is the standard bulk envelope payload.
type BulkResponse[T any] struct {
	Results []BulkResult[T] `json:"results"`
	Summary BulkSummary     `json:"summary"`
}

// Empty is the data type for delete-style bulk results that carry no
// payload beyond the per-item status.
type Empty struct{}
