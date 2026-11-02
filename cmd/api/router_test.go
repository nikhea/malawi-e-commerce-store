package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/internal/cart"
	"github.com/nikhea/malawi-e-commerce-store/internal/inventory"
	"github.com/nikhea/malawi-e-commerce-store/internal/products"
	"github.com/nikhea/malawi-e-commerce-store/internal/reviews"
	"github.com/nikhea/malawi-e-commerce-store/internal/variants"
	"github.com/nikhea/malawi-e-commerce-store/internal/wishlist"
	"github.com/nikhea/malawi-e-commerce-store/pkg/middleware"
)

// testRouter mirrors cmd/api's grouping closely enough to exercise route
// order and the admin gate. Handlers get nil services on purpose: every
// request here is rejected before a handler runs.
func testRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	open := r.Group("/api/v1")

	protected := r.Group("/api/v1")
	// Stand-in for JWT: the test headers carry identity + role.
	protected.Use(func(c *gin.Context) {
		if role := c.GetHeader("X-Test-Role"); role != "" {
			middleware.SetRole(c, role)
		}
		if user := c.GetHeader("X-Test-User"); user != "" {
			c.Set("user_id", user)
		}
		c.Next()
	})

	inventory.RegisterRoutes(protected, nil, nil)
	products.RegisterRoutes(open, protected, nil, nil, nil)
	variants.RegisterRoutes(open, protected, nil, nil)
	cart.RegisterRoutes(protected, nil, nil)
	wishlist.RegisterRoutes(protected, nil, nil)
	reviews.RegisterRoutes(open, protected, nil)
	return r
}

// TestBulkRoutesRegistered locks the route table: every bulk path exists and
// the static segments are registered next to the id routes.
func TestBulkRoutesRegistered(t *testing.T) {
	r := testRouter()
	got := make(map[string]bool, len(r.Routes()))
	for _, route := range r.Routes() {
		got[route.Method+" "+route.Path] = true
	}

	want := []string{
		http.MethodPost + " /api/v1/admin/inventory/stock/bulk",
		http.MethodGet + " /api/v1/admin/inventory/stock",
		http.MethodPost + " /api/v1/admin/products/bulk",
		http.MethodPatch + " /api/v1/admin/products/bulk",
		http.MethodPost + " /api/v1/admin/products/bulk-delete",
		http.MethodGet + " /api/v1/products",
		http.MethodPost + " /api/v1/products/:id/variants/bulk",
		http.MethodPatch + " /api/v1/admin/variants/bulk",
		http.MethodPost + " /api/v1/cart/items/bulk",
		http.MethodGet + " /api/v1/reviews/me",
		http.MethodPost + " /api/v1/wishlist/bulk",
		http.MethodPost + " /api/v1/wishlist/bulk-delete",
	}
	for _, route := range want {
		if !got[route] {
			t.Errorf("route not registered: %s", route)
		}
	}
}

// TestAdminBulkRoutesForbidCustomers proves every admin bulk route sits
// behind RequireRole. 403 must come from the middleware, before any handler
// touches its nil service.
func TestAdminBulkRoutesForbidCustomers(t *testing.T) {
	r := testRouter()
	routes := []struct{ method, path string }{
		{http.MethodPost, "/api/v1/admin/inventory/stock/bulk"},
		{http.MethodGet, "/api/v1/admin/inventory/stock?variantIds=v1"},
		{http.MethodPost, "/api/v1/admin/products/bulk"},
		{http.MethodPatch, "/api/v1/admin/products/bulk"},
		{http.MethodPost, "/api/v1/admin/products/bulk-delete"},
		{http.MethodPost, "/api/v1/products/p1/variants/bulk"},
		{http.MethodPatch, "/api/v1/admin/variants/bulk"},
	}
	for _, rt := range routes {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(rt.method, rt.path, strings.NewReader(`{}`))
		req.Header.Set("X-Test-Role", "customer")
		req.Header.Set("X-Test-User", "u1")
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("%s %s: expected 403, got %d: %s", rt.method, rt.path, w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "FORBIDDEN") {
			t.Errorf("%s %s: missing FORBIDDEN code: %s", rt.method, rt.path, w.Body.String())
		}
	}
}

// TestAdminBulkRoutesRequireIdentity: no role at all is 401, not 403.
func TestAdminBulkRoutesRequireIdentity(t *testing.T) {
	r := testRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products/bulk", strings.NewReader(`{}`))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}
