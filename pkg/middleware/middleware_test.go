package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/pkg/middleware"
)

func requestIDFor(t *testing.T, header string) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.RequestID())
	r.GET("/x", func(c *gin.Context) {
		c.String(http.StatusOK, middleware.RequestIDOf(c))
	})
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	if header != "" {
		req.Header.Set("X-Request-ID", header)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Body.String()
}

func TestRequestIDRejectsLogInjection(t *testing.T) {
	// A newline in a caller-supplied id would let an attacker forge log
	// lines; the middleware must strip anything not safe in a header.
	got := requestIDFor(t, "abc\ninjected=yes")
	if strings.ContainsAny(got, "\n\r") {
		t.Fatalf("control characters survived: %q", got)
	}
	if got != "abcinjectedyes" {
		t.Fatalf("unexpected sanitized id: %q", got)
	}
}

func TestRequestIDIsBounded(t *testing.T) {
	got := requestIDFor(t, strings.Repeat("a", 500))
	if len(got) != 64 {
		t.Fatalf("expected 64-char cap, got %d", len(got))
	}
}

func TestRequestIDGeneratesWhenAbsent(t *testing.T) {
	got := requestIDFor(t, "")
	if got == "" {
		t.Fatal("no request id generated")
	}
}
