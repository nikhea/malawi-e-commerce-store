package test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/internal/wishlist"
	"github.com/nikhea/malawi-e-commerce-store/internal/wishlist/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/bulk"
	"github.com/nikhea/malawi-e-commerce-store/pkg/idempotency"
	"github.com/redis/go-redis/v9"
)

// fakeService records calls so handler tests can prove identity scoping and
// idempotent replay without a database.
type fakeService struct {
	addCalls    int
	removeCalls int
	lastUserID  string
}

func (f *fakeService) Add(context.Context, string, string) (public.Item, error) {
	return public.Item{}, nil
}

func (f *fakeService) Remove(context.Context, string, string) error { return nil }

func (f *fakeService) ListMine(context.Context, string, public.ListFilter) (public.Page, error) {
	return public.Page{}, nil
}

func (f *fakeService) BulkAdd(_ context.Context, userID string, productIDs []string, _ bool) ([]bulk.BulkResult[public.Item], error) {
	f.addCalls++
	f.lastUserID = userID
	results := make([]bulk.BulkResult[public.Item], len(productIDs))
	for i, id := range productIDs {
		results[i] = bulk.OK(i, id, public.Item{ProductID: id})
	}
	return results, nil
}

func (f *fakeService) BulkRemove(_ context.Context, userID string, productIDs []string, _, dryRun bool) ([]bulk.BulkResult[bulk.Empty], error) {
	f.removeCalls++
	f.lastUserID = userID
	results := make([]bulk.BulkResult[bulk.Empty], len(productIDs))
	for i, id := range productIDs {
		results[i] = bulk.OK[bulk.Empty](i, id, bulk.Empty{})
		results[i].WouldDelete = dryRun
	}
	return results, nil
}

// router mounts the real wishlist route table under /api/v1 with a stand-in
// for the JWT middleware: the X-Test-User header fills the caller id.
func router(svc public.Service, store *idempotency.Store) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	group := r.Group("/api/v1")
	group.Use(func(c *gin.Context) {
		if user := c.GetHeader("X-Test-User"); user != "" {
			c.Set("user_id", user)
		}
		c.Next()
	})
	wishlist.RegisterRoutes(group, svc, store)
	return r
}

func do(r http.Handler, method, path, user, key, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if user != "" {
		req.Header.Set("X-Test-User", user)
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	r.ServeHTTP(w, req)
	return w
}

func newIdemStore(t *testing.T) *idempotency.Store {
	t.Helper()
	mr := miniredis.RunT(t)
	return idempotency.New(redis.NewClient(&redis.Options{Addr: mr.Addr()}))
}

func TestBulkAddUsesTokenIdentity(t *testing.T) {
	svc := &fakeService{}
	r := router(svc, nil)

	// A body-supplied user id is not part of the DTO and must be ignored.
	w := do(r, http.MethodPost, "/api/v1/wishlist/bulk", "caller-1", "",
		`{"productIds":["p1"],"user_id":"attacker"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if svc.lastUserID != "caller-1" {
		t.Fatalf("service got user %q, want caller-1", svc.lastUserID)
	}
	if !strings.Contains(w.Body.String(), `"results"`) || !strings.Contains(w.Body.String(), `"summary"`) {
		t.Fatalf("missing bulk envelope: %s", w.Body.String())
	}
}

func TestStaticBulkPathsBeatParams(t *testing.T) {
	svc := &fakeService{}
	r := router(svc, nil)

	// /wishlist/bulk must reach BulkAdd, not fall through to /wishlist or a
	// future /wishlist/{id} POST.
	if w := do(r, http.MethodPost, "/api/v1/wishlist/bulk", "u1", "", `{"productIds":["p1"]}`); w.Code != http.StatusOK {
		t.Fatalf("bulk path: expected 200, got %d", w.Code)
	}
	// /wishlist/bulk-delete must reach BulkRemove.
	w := do(r, http.MethodPost, "/api/v1/wishlist/bulk-delete", "u1", "", `{"ids":["p1"],"dryRun":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("bulk-delete path: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"wouldDelete":true`) {
		t.Fatalf("dryRun flag not surfaced: %s", w.Body.String())
	}
	if svc.addCalls != 1 || svc.removeCalls != 1 {
		t.Fatalf("wrong handlers ran: add=%d remove=%d", svc.addCalls, svc.removeCalls)
	}
}

func TestIdempotencyReplayRealHandler(t *testing.T) {
	svc := &fakeService{}
	r := router(svc, newIdemStore(t))
	body := `{"productIds":["p1","p2"]}`

	first := do(r, http.MethodPost, "/api/v1/wishlist/bulk", "u1", "k1", body)
	second := do(r, http.MethodPost, "/api/v1/wishlist/bulk", "u1", "k1", body)
	if first.Code != http.StatusOK || second.Code != http.StatusOK {
		t.Fatalf("expected 200 twice, got %d %d", first.Code, second.Code)
	}
	if first.Body.String() != second.Body.String() {
		t.Fatalf("replay body differs:\n%s\n%s", first.Body.String(), second.Body.String())
	}
	if svc.addCalls != 1 {
		t.Fatalf("side effect ran %d times, want 1", svc.addCalls)
	}

	// Same key, different body: reject without running the handler.
	conflict := do(r, http.MethodPost, "/api/v1/wishlist/bulk", "u1", "k1", `{"productIds":["p3"]}`)
	if conflict.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", conflict.Code, conflict.Body.String())
	}
	if !strings.Contains(conflict.Body.String(), "IDEMPOTENCY_CONFLICT") {
		t.Fatalf("missing IDEMPOTENCY_CONFLICT: %s", conflict.Body.String())
	}
	if svc.addCalls != 1 {
		t.Fatalf("conflicting request ran the handler: %d", svc.addCalls)
	}
}

func TestIdempotencyScopedPerUser(t *testing.T) {
	svc := &fakeService{}
	r := router(svc, newIdemStore(t))
	body := `{"productIds":["p1"]}`

	alice := do(r, http.MethodPost, "/api/v1/wishlist/bulk", "alice", "shared", body)
	bob := do(r, http.MethodPost, "/api/v1/wishlist/bulk", "bob", "shared", body)
	if alice.Code != http.StatusOK || bob.Code != http.StatusOK {
		t.Fatalf("expected 200 twice, got %d %d", alice.Code, bob.Code)
	}
	if svc.addCalls != 2 {
		t.Fatalf("same key must be independent per user, ran %d times", svc.addCalls)
	}
}

func TestRequestWithoutKeyAlwaysRuns(t *testing.T) {
	svc := &fakeService{}
	r := router(svc, newIdemStore(t))

	for i := 0; i < 3; i++ {
		if w := do(r, http.MethodPost, "/api/v1/wishlist/bulk", "u1", "", `{"productIds":["p1"]}`); w.Code != http.StatusOK {
			t.Fatalf("request %d: %d", i, w.Code)
		}
	}
	if svc.addCalls != 3 {
		t.Fatalf("expected every request to run, got %d", svc.addCalls)
	}
}
