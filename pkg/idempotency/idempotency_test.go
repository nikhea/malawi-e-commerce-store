package idempotency_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	"github.com/nikhea/malawi-e-commerce-store/pkg/idempotency"
	"github.com/nikhea/malawi-e-commerce-store/pkg/response"
	"github.com/redis/go-redis/v9"
)

func setup(t *testing.T) (*miniredis.Miniredis, *idempotency.Store, *int32) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := idempotency.New(client)
	var calls int32
	return mr, store, &calls
}

// router wires an optional user id and a handler that counts executions.
func router(store *idempotency.Store, user string, calls *int32) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if user != "" {
			c.Set("user_id", user)
		}
		c.Next()
	}, store.Middleware())
	r.POST("/bulk", func(c *gin.Context) {
		n := atomic.AddInt32(calls, 1)
		response.OK(c, gin.H{"n": n})
	})
	r.POST("/fail", func(c *gin.Context) {
		atomic.AddInt32(calls, 1)
		response.Error(c, apperr.Validation("bad request"))
	})
	return r
}

func do(r http.Handler, path, key, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	r.ServeHTTP(w, req)
	return w
}

func TestReplaySameKeyAndBody(t *testing.T) {
	_, store, calls := setup(t)
	r := router(store, "u1", calls)

	first := do(r, "/bulk", "k1", `{"a":1}`)
	second := do(r, "/bulk", "k1", `{"a":1}`)

	if first.Code != http.StatusOK || second.Code != http.StatusOK {
		t.Fatalf("expected 200 twice, got %d %d", first.Code, second.Code)
	}
	if atomic.LoadInt32(calls) != 1 {
		t.Fatalf("handler ran %d times, expected 1", *calls)
	}
	if first.Body.String() != second.Body.String() {
		t.Fatalf("replay body differs: %q vs %q", first.Body.String(), second.Body.String())
	}
}

func TestSameKeyDifferentBodyConflicts(t *testing.T) {
	_, store, calls := setup(t)
	r := router(store, "u1", calls)

	if w := do(r, "/bulk", "k1", `{"a":1}`); w.Code != http.StatusOK {
		t.Fatalf("first: %d", w.Code)
	}
	w := do(r, "/bulk", "k1", `{"a":2}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), string(apperr.CodeIdempotencyConflict)) {
		t.Fatalf("missing IDEMPOTENCY_CONFLICT: %s", w.Body.String())
	}
	if atomic.LoadInt32(calls) != 1 {
		t.Fatalf("conflicting request executed the handler: %d", *calls)
	}
}

func TestDifferentPathsWithSameKeyConflict(t *testing.T) {
	_, store, calls := setup(t)
	r := router(store, "u1", calls)

	if w := do(r, "/bulk", "k1", `{}`); w.Code != http.StatusOK {
		t.Fatalf("first: %d", w.Code)
	}
	if w := do(r, "/fail", "k1", `{}`); w.Code != http.StatusConflict {
		t.Fatalf("expected 409 across endpoints, got %d", w.Code)
	}
}

func TestKeysAreScopedPerUser(t *testing.T) {
	_, store, calls := setup(t)

	alice := router(store, "alice", calls)
	bob := router(store, "bob", calls)

	if w := do(alice, "/bulk", "shared", `{}`); w.Code != http.StatusOK {
		t.Fatalf("alice: %d", w.Code)
	}
	if w := do(bob, "/bulk", "shared", `{}`); w.Code != http.StatusOK {
		t.Fatalf("bob: %d", w.Code)
	}
	if atomic.LoadInt32(calls) != 2 {
		t.Fatalf("expected two executions, got %d", *calls)
	}
}

func TestNon2xxIsNotCached(t *testing.T) {
	_, store, calls := setup(t)
	r := router(store, "u1", calls)

	for i := 0; i < 2; i++ {
		if w := do(r, "/fail", "k1", `{}`); w.Code != http.StatusBadRequest {
			t.Fatalf("attempt %d: expected 400, got %d", i, w.Code)
		}
	}
	if atomic.LoadInt32(calls) != 2 {
		t.Fatalf("errors must not be cached, ran %d times", *calls)
	}
}

func TestNoKeySkipsCache(t *testing.T) {
	_, store, calls := setup(t)
	r := router(store, "u1", calls)

	for i := 0; i < 3; i++ {
		if w := do(r, "/bulk", "", `{}`); w.Code != http.StatusOK {
			t.Fatalf("request %d: %d", i, w.Code)
		}
	}
	if atomic.LoadInt32(calls) != 3 {
		t.Fatalf("expected every request to run, got %d", *calls)
	}
}

func TestTooLongKeyRejected(t *testing.T) {
	_, store, calls := setup(t)
	r := router(store, "u1", calls)

	w := do(r, "/bulk", strings.Repeat("x", 201), `{}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if atomic.LoadInt32(calls) != 0 {
		t.Fatal("handler ran for a rejected key")
	}
}

func TestFailOpenWhenRedisIsDown(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	store := idempotency.New(client)
	var calls int32
	r := router(store, "u1", &calls)

	for i := 0; i < 2; i++ {
		if w := do(r, "/bulk", "k1", `{}`); w.Code != http.StatusOK {
			t.Fatalf("fail-open request %d: %d", i, w.Code)
		}
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("expected 2 runs with dead redis, got %d", calls)
	}
}

func TestNilStorePassesThrough(t *testing.T) {
	var store *idempotency.Store
	var calls int32
	r := router(store, "u1", &calls)

	if w := do(r, "/bulk", "k1", `{}`); w.Code != http.StatusOK {
		t.Fatalf("nil store: %d", w.Code)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("nil store must not dedupe, got %d", calls)
	}
}

func TestCacheTTLIs24Hours(t *testing.T) {
	mr, store, calls := setup(t)
	r := router(store, "u1", calls)

	if w := do(r, "/bulk", "k1", `{}`); w.Code != http.StatusOK {
		t.Fatalf("first: %d", w.Code)
	}
	ttl := mr.TTL("idem:v1:u1:k1")
	if ttl < 23*time.Hour || ttl > 24*time.Hour {
		t.Fatalf("expected ~24h ttl, got %s", ttl)
	}
}

func TestConcurrentDuplicateIsRejected(t *testing.T) {
	mr, store, calls := setup(t)
	r := router(store, "u1", calls)

	// Simulate the first request still running: the middleware holds this
	// lock until the handler returns and the response is cached.
	mr.Set("idem:v1:u1:k1:lock", "1")

	w := do(r, "/bulk", "k1", `{}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 for an in-flight key, got %d: %s", w.Code, w.Body.String())
	}
	if atomic.LoadInt32(calls) != 0 {
		t.Fatalf("in-flight duplicate executed the handler: %d", *calls)
	}
}

func TestLockIsReleasedAfterRequest(t *testing.T) {
	mr, store, calls := setup(t)
	r := router(store, "u1", calls)

	if w := do(r, "/bulk", "k1", `{}`); w.Code != http.StatusOK {
		t.Fatalf("first: %d", w.Code)
	}
	if mr.Exists("idem:v1:u1:k1:lock") {
		t.Fatal("lock leaked after the request finished")
	}
}

func TestOversizedBodyRejected(t *testing.T) {
	_, store, calls := setup(t)
	r := router(store, "u1", calls)

	big := `{"pad":"` + strings.Repeat("a", 1<<20) + `"}`
	w := do(r, "/bulk", "k1", big)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an oversized idempotent body, got %d", w.Code)
	}
	if atomic.LoadInt32(calls) != 0 {
		t.Fatalf("oversized body executed the handler: %d", *calls)
	}
}
