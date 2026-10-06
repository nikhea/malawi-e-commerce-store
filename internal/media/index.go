package media

import (
	"github.com/jackc/pgx/v5"
	"github.com/nikhea/malawi-e-commerce-store/internal/media/service"
	pkgmedia "github.com/nikhea/malawi-e-commerce-store/pkg/media"
	"github.com/riverqueue/river"
)

// Wire builds the media pipeline around an already-built River client and
// Cloudinary uploader. Returns the concrete service (not the interface)
// because the wiring site also registers it as a River worker and attaches
// per-owner completion callbacks. This file is the module's single entry
// point — cmd/* calls Wire instead of assembling the layers by hand.
func Wire(client *river.Client[pgx.Tx], uploader pkgmedia.Uploader) *service.Service {
	return service.NewService(client, uploader)
}
