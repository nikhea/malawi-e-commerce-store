package media

import (
	"context"
	"fmt"
	"io"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

// Result is the outcome of one upload. Domain-free: owners store URL,
// PublicID lets them overwrite/delete the asset later.
type Result struct {
	URL      string
	PublicID string
}

// Uploader moves bytes to Cloudinary. Interface-kept so workers and
// handlers test against a fake; the app wires the Cloudinary impl once.
type Uploader interface {
	// Upload sends r (filename used for format detection) into folder.
	Upload(ctx context.Context, r io.Reader, filename, folder string) (Result, error)
}

// CloudinaryConfig carries the credentials. Assembled from config.Config
// at the wiring site; this package never reads env.
type CloudinaryConfig struct {
	CloudName string
	APIKey    string
	APISecret string
}

type cloudinaryUploader struct {
	cld *cloudinary.Cloudinary
}

// NewCloudinaryUploader builds the production uploader. Fails fast on bad
// credentials instead of failing the first upload at runtime.
func NewCloudinaryUploader(cfg CloudinaryConfig) (Uploader, error) {
	if cfg.CloudName == "" || cfg.APIKey == "" || cfg.APISecret == "" {
		return nil, fmt.Errorf("cloudinary: cloud name, api key and secret are required")
	}
	cld, err := cloudinary.NewFromParams(cfg.CloudName, cfg.APIKey, cfg.APISecret)
	if err != nil {
		return nil, fmt.Errorf("cloudinary: %w", err)
	}
	return &cloudinaryUploader{cld: cld}, nil
}

func (u *cloudinaryUploader) Upload(ctx context.Context, r io.Reader, filename, folder string) (Result, error) {
	res, err := u.cld.Upload.Upload(ctx, r, uploader.UploadParams{
		Folder:           folder,
		FilenameOverride: filename,
	})
	if err != nil {
		return Result{}, fmt.Errorf("cloudinary upload: %w", err)
	}
	return Result{URL: res.SecureURL, PublicID: res.PublicID}, nil
}
