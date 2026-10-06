// Package public is the media module's external contract. It is the ONLY
// media package other modules and cmd/* may import. Any module that needs
// an async upload (categories now; products, avatars later) goes through
// here: enqueue a job, register a completion callback, done.
package public

import "context"

// Owner types. A new image owner (product, avatar, …) adds one constant
// here and one callback at the wiring site — no other media changes.
const (
	OwnerCategory = "category"
)

// CompleteFunc stores the upload outcome on the owning record. Registered
// per owner type; the worker calls it after a successful upload.
type CompleteFunc func(ctx context.Context, ownerID, url, publicID string) error

// EnqueueInput describes one async upload. Exactly one source: TempPath
// (multipart saved to disk by the handler; worker deletes it) or
// SourceURL (worker fetches it).
type EnqueueInput struct {
	OwnerType string
	OwnerID   string
	Filename  string
	Folder    string
	TempPath  string
	SourceURL string
}

// Service is the media capability surface.
type Service interface {
	// Enqueue validates and inserts the upload job. The API answers
	// immediately (202); Cloudinary + the callback happen in River.
	Enqueue(ctx context.Context, in EnqueueInput) error
	// RegisterComplete sets the post-upload callback for an owner type.
	// Called once at wiring, before the River client starts.
	RegisterComplete(ownerType string, fn CompleteFunc)
}
