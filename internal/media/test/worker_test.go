package test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	mediapublic "github.com/nikhea/malawi-e-commerce-store/internal/media/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/media/service"
	pkgmedia "github.com/nikhea/malawi-e-commerce-store/pkg/media"
	"github.com/riverqueue/river"
)

// fakeUploader records calls and returns a fixed result. No Cloudinary.
type fakeUploader struct {
	folder   string
	filename string
	calls    int
}

func (f *fakeUploader) Upload(_ context.Context, r io.Reader, filename, folder string) (pkgmedia.Result, error) {
	f.calls++
	f.folder = folder
	f.filename = filename
	if _, err := io.ReadAll(r); err != nil {
		return pkgmedia.Result{}, err
	}
	return pkgmedia.Result{URL: "https://cdn/test.jpg", PublicID: "test"}, nil
}

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "img.jpg")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("temp file: %v", err)
	}
	return p
}

// newWorkService builds the pipeline without a River client: Work never
// touches the client (only Enqueue does), so nil is safe here.
func newWorkService(u pkgmedia.Uploader) *service.Service {
	return service.NewService(nil, u)
}

func TestWorkTempFile(t *testing.T) {
	ctx := context.Background()
	uploader := &fakeUploader{}
	svc := newWorkService(uploader)

	var gotID, gotURL string
	svc.RegisterComplete(mediapublic.OwnerCategory,
		func(_ context.Context, ownerID, url, _ string) error {
			gotID, gotURL = ownerID, url
			return nil
		})

	tmp := writeTemp(t, "fake-bytes")
	job := &river.Job[service.UploadArgs]{Args: service.UploadArgs{
		OwnerType: mediapublic.OwnerCategory,
		OwnerID:   "cat-1",
		Filename:  "oil.jpg",
		Folder:    "malawi-store/categories",
		TempPath:  tmp,
	}}
	if err := svc.Work(ctx, job); err != nil {
		t.Fatalf("work: %v", err)
	}
	if uploader.calls != 1 || uploader.folder != "malawi-store/categories" {
		t.Fatalf("uploader not called correctly: %+v", uploader)
	}
	if gotID != "cat-1" || gotURL != "https://cdn/test.jpg" {
		t.Fatalf("callback args wrong: %q %q", gotID, gotURL)
	}
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Fatalf("temp file not cleaned up: %s", tmp)
	}
}

func TestWorkFailures(t *testing.T) {
	ctx := context.Background()

	t.Run("missing temp file errors", func(t *testing.T) {
		svc := newWorkService(&fakeUploader{})
		svc.RegisterComplete(mediapublic.OwnerCategory, func(context.Context, string, string, string) error { return nil })
		job := &river.Job[service.UploadArgs]{Args: service.UploadArgs{
			OwnerType: mediapublic.OwnerCategory, OwnerID: "cat-1", TempPath: "/nope/missing.jpg",
		}}
		if err := svc.Work(ctx, job); err == nil {
			t.Fatal("expected error for missing file")
		}
	})

	t.Run("unregistered owner errors", func(t *testing.T) {
		svc := newWorkService(&fakeUploader{})
		tmp := writeTemp(t, "x")
		job := &river.Job[service.UploadArgs]{Args: service.UploadArgs{
			OwnerType: "spaceship", OwnerID: "s-1", TempPath: tmp,
		}}
		if err := svc.Work(ctx, job); err == nil {
			t.Fatal("expected error for unknown owner")
		}
	})
}
