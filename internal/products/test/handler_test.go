package test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	mediapublic "github.com/nikhea/malawi-e-commerce-store/internal/media/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/products/handler"
	"github.com/nikhea/malawi-e-commerce-store/internal/products/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/products/service"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
)

type fakeMedia struct {
	inputs []mediapublic.EnqueueInput
	fail   error
}

func (f *fakeMedia) Enqueue(_ context.Context, in mediapublic.EnqueueInput) error {
	if f.fail != nil {
		return f.fail
	}
	f.inputs = append(f.inputs, in)
	return nil
}

func (f *fakeMedia) RegisterComplete(string, mediapublic.CompleteFunc) {}

type uploadEnvelope struct {
	Data struct {
		Status string `json:"status"`
		Queued int    `json:"queued"`
		Failed int    `json:"failed"`
	} `json:"data"`
}

func uploadRequest(t *testing.T, url string, names ...string) *http.Request {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for _, name := range names {
		part, err := writer.CreateFormFile("files", name)
		if err != nil {
			t.Fatalf("form file: %v", err)
		}
		if _, err := part.Write([]byte("fake-image-bytes")); err != nil {
			t.Fatalf("write part: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, url, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestUploadImagesQueuesOneJobPerFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newFakeRepo(model.Product{ID: "prod-oil", Name: "Oil", Slug: "oil", PriceCents: 100})
	svc := service.NewService(repo, fakeCategories{}, fakeVariants{})
	media := &fakeMedia{}

	router := gin.New()
	router.POST("/api/v1/admin/products/:id/images", handler.New(svc, media).UploadImages)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, uploadRequest(t, "/api/v1/admin/products/prod-oil/images", "one.jpg", "two.jpg"))

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	var env uploadEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Data.Status != "queued" || env.Data.Queued != 2 || env.Data.Failed != 0 {
		t.Fatalf("bad response: %+v", env.Data)
	}
	if len(media.inputs) != 2 {
		t.Fatalf("expected 2 media jobs, got %+v", media.inputs)
	}
	for _, in := range media.inputs {
		if in.OwnerType != mediapublic.OwnerProductImage || in.OwnerID == "" {
			t.Fatalf("job must target the image row: %+v", in)
		}
		if in.Folder != "malawi-store/products" || in.Filename == "" || in.TempPath == "" {
			t.Fatalf("job missing upload data: %+v", in)
		}
	}
	if got := len(repo.images["prod-oil"]); got != 2 {
		t.Fatalf("expected 2 pending rows, got %d", got)
	}
}

func TestUploadImagesRollsBackRowsWhenQueueingFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newFakeRepo(model.Product{ID: "prod-oil", Name: "Oil", Slug: "oil", PriceCents: 100})
	svc := service.NewService(repo, fakeCategories{}, fakeVariants{})
	media := &fakeMedia{fail: apperr.Internal(errors.New("river down"))}

	router := gin.New()
	router.POST("/api/v1/admin/products/:id/images", handler.New(svc, media).UploadImages)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, uploadRequest(t, "/api/v1/admin/products/prod-oil/images", "one.jpg"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := len(repo.images["prod-oil"]); got != 0 {
		t.Fatalf("failed upload left %d rows behind", got)
	}
}

func TestUploadImagesRequiresFiles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newFakeRepo(model.Product{ID: "prod-oil", Name: "Oil", Slug: "oil", PriceCents: 100})
	svc := service.NewService(repo, fakeCategories{}, fakeVariants{})

	router := gin.New()
	router.POST("/api/v1/admin/products/:id/images", handler.New(svc, &fakeMedia{}).UploadImages)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products/prod-oil/images", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}
