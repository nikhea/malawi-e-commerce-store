package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	mediapublic "github.com/nikhea/malawi-e-commerce-store/internal/media/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	pkgmedia "github.com/nikhea/malawi-e-commerce-store/pkg/media"
	"github.com/nikhea/malawi-e-commerce-store/pkg/ssrf"
	"github.com/riverqueue/river"
)

var _ mediapublic.Service = (*Service)(nil)

// maxUploadBytes caps a single image (multipart or fetched) at 10 MiB.
const maxUploadBytes = 10 << 20

// UploadArgs is the River job payload. JSON-serializable by necessity:
// no readers, no funcs, no interfaces — just data.
type UploadArgs struct {
	OwnerType string `json:"owner_type"`
	OwnerID   string `json:"owner_id"`
	Filename  string `json:"filename"`
	Folder    string `json:"folder"`
	TempPath  string `json:"temp_path"`
	SourceURL string `json:"source_url"`
}

// Kind identifies the job in River. One kind serves all owners.
func (UploadArgs) Kind() string { return "media_upload" }

// Service enqueues uploads and works them. The SAME struct implements both
// mediapublic.Service (Enqueue, called by handlers) and
// river.Worker[UploadArgs] (Work, called by River) — one object, two roles.
type Service struct {
	river.WorkerDefaults[UploadArgs]
	client    *river.Client[pgx.Tx]
	uploader  pkgmedia.Uploader
	callbacks map[string]mediapublic.CompleteFunc
	http      *http.Client
}

// NewService builds the media pipeline. client is the started River
// client; uploader performs the Cloudinary calls; callbacks are empty
// until RegisterComplete fills them at wiring.
func NewService(client *river.Client[pgx.Tx], uploader pkgmedia.Uploader) *Service {
	return &Service{
		client:    client,
		uploader:  uploader,
		callbacks: map[string]mediapublic.CompleteFunc{},
		http:      &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *Service) RegisterComplete(ownerType string, fn mediapublic.CompleteFunc) {
	s.callbacks[ownerType] = fn
}

func (s *Service) Enqueue(ctx context.Context, in mediapublic.EnqueueInput) error {
	if strings.TrimSpace(in.OwnerType) == "" || strings.TrimSpace(in.OwnerID) == "" {
		return apperr.Validation("owner type and id are required")
	}
	if in.TempPath == "" && strings.TrimSpace(in.SourceURL) == "" {
		return apperr.Validation("temp file or source url is required")
	}
	if _, err := s.client.Insert(ctx, UploadArgs{
		OwnerType: in.OwnerType,
		OwnerID:   in.OwnerID,
		Filename:  in.Filename,
		Folder:    in.Folder,
		TempPath:  in.TempPath,
		SourceURL: strings.TrimSpace(in.SourceURL),
	}, nil); err != nil {
		return apperr.Internal(fmt.Errorf("enqueue upload: %w", err))
	}
	return nil
}

// Work downloads/fetches the source, uploads to Cloudinary, cleans up,
// then fires the owner's callback. Any returned error retries the job
// with River's backoff — so callbacks must be idempotent.
func (s *Service) Work(ctx context.Context, job *river.Job[UploadArgs]) error {
	args := job.Args

	reader, cleanup, err := s.source(ctx, args)
	if err != nil {
		return err
	}
	defer cleanup()

	res, err := s.uploader.Upload(ctx, reader, args.Filename, args.Folder)
	if err != nil {
		return fmt.Errorf("upload %s/%s: %w", args.OwnerType, args.OwnerID, err)
	}

	fn, ok := s.callbacks[args.OwnerType]
	if !ok {
		return fmt.Errorf("no callback registered for owner type %q", args.OwnerType)
	}
	if err := fn(ctx, args.OwnerID, res.URL, res.PublicID); err != nil {
		return fmt.Errorf("complete %s/%s: %w", args.OwnerType, args.OwnerID, err)
	}
	return nil
}

// source resolves the job to a size-capped reader plus its cleanup.
func (s *Service) source(ctx context.Context, args UploadArgs) (io.Reader, func(), error) {
	noop := func() {}
	if args.TempPath != "" {
		f, err := os.Open(args.TempPath)
		if err != nil {
			return nil, noop, fmt.Errorf("open temp file: %w", err)
		}
		cleanup := func() {
			f.Close()
			os.Remove(args.TempPath)
		}
		if st, err := f.Stat(); err == nil && st.Size() > maxUploadBytes {
			cleanup()
			return nil, noop, river.JobCancel(apperr.Validation("image exceeds 10 MiB"))
		}
		return f, cleanup, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, args.SourceURL, nil)
	if err != nil {
		return nil, noop, fmt.Errorf("build fetch request: %w", err)
	}
	// SSRF: the URL may be attacker-influenced (future public upload
	// paths). Deterministic failure → cancel, never retry.
	if err := ssrf.ValidateURL(ctx, args.SourceURL); err != nil {
		return nil, noop, river.JobCancel(err)
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, noop, fmt.Errorf("fetch source: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, noop, fmt.Errorf("fetch source: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxUploadBytes+1))
	if err != nil {
		return nil, noop, fmt.Errorf("read source: %w", err)
	}
	if int64(len(body)) > maxUploadBytes {
		// Deterministic failure: retrying can never succeed, so cancel
		// the job instead of burning through River's backoff.
		return nil, noop, river.JobCancel(apperr.Validation("image exceeds 10 MiB"))
	}
	return bytes.NewReader(body), noop, nil
}
