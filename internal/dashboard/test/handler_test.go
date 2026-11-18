package test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/internal/dashboard"
	"github.com/nikhea/malawi-e-commerce-store/internal/dashboard/public"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/middleware"
)

type fakeService struct {
	calls int
}

func (f *fakeService) Overview(context.Context) (public.Overview, error) {
	f.calls++
	return public.Overview{
		Catalog: public.Catalog{TotalProducts: 12},
		Orders:  public.Orders{Total: 9, Pending: 2, Paid: 6, Cancelled: 1},
		Demand:  public.Demand{MostOrdered: []public.OrderedProduct{{ProductID: "p1"}}},
	}, nil
}

func testRouter(svc public.Service) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	protected := r.Group("/api/v1")
	protected.Use(func(c *gin.Context) {
		if role := c.GetHeader("X-Test-Role"); role != "" {
			middleware.SetRole(c, role)
		}
		if user := c.GetHeader("X-Test-User"); user != "" {
			c.Set("user_id", user)
		}
		c.Next()
	})
	dashboard.RegisterRoutes(protected, svc)
	return r
}

func TestAdminGetsOverview(t *testing.T) {
	svc := &fakeService{}
	r := testRouter(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard/overview", nil)
	req.Header.Set("X-Test-Role", string(userspublic.RoleAdmin))
	req.Header.Set("X-Test-User", "admin-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if svc.calls != 1 {
		t.Fatalf("service called %d times, want 1", svc.calls)
	}
	for _, field := range []string{
		`"catalog"`, `"customers"`, `"orders"`, `"fulfillment"`, `"stock"`,
		`"reviews"`, `"payments"`, `"demand"`, `"generated_at"`,
		`"paying_customers"`, `"most_wishlisted"`,
	} {
		if !strings.Contains(w.Body.String(), field) {
			t.Errorf("missing %s in response: %s", field, w.Body.String())
		}
	}
}

func TestOverviewRoleGate(t *testing.T) {
	r := testRouter(&fakeService{})

	t.Run("customer forbidden", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard/overview", nil)
		req.Header.Set("X-Test-Role", string(userspublic.RoleCustomer))
		req.Header.Set("X-Test-User", "u1")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "FORBIDDEN") {
			t.Fatalf("expected 403 FORBIDDEN, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("no identity unauthorized", func(t *testing.T) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard/overview", nil))
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestOverviewRouteRegistered(t *testing.T) {
	for _, route := range testRouter(&fakeService{}).Routes() {
		if route.Method == http.MethodGet && route.Path == "/api/v1/admin/dashboard/overview" {
			return
		}
	}
	t.Fatal("GET /api/v1/admin/dashboard/overview is not registered")
}
