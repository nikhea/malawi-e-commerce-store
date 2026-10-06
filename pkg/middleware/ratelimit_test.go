package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/pkg/middleware"
	"github.com/redis/go-redis/v9"
)

func testRouter(client *redis.Client) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.RateLimit(client, middleware.RateLimitConfig{
		Name: "test", Limit: 3, Window: time.Minute,
	}))
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })
	return r
}

func TestRateLimitBlocksOverLimit(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	r := testRouter(client)

	var last int
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		r.ServeHTTP(w, req)
		last = w.Code
		if i < 3 && w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, w.Code)
		}
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("expected final 429, got %d", last)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	r.ServeHTTP(w, req)
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("missing Retry-After header on 429")
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("expected JSON envelope, got %q", ct)
	}
}

func TestRateLimitPerIP(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	r := testRouter(client)

	// Exhaust one IP; another IP still passes.
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.RemoteAddr = "10.0.0.9:1234"
		r.ServeHTTP(w, req)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.RemoteAddr = "10.0.0.10:1234"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("different IP blocked: %d", w.Code)
	}
}

func TestRateLimitFailOpen(t *testing.T) {
	// Nothing listening here: every Redis call errors.
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.RateLimit(client, middleware.RateLimitConfig{
		Name: "test", Limit: 1, Window: time.Minute,
	}))
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })

	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("fail-open broken on request %d: %d", i, w.Code)
		}
	}
}
