package test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/internal/orders"
	"github.com/nikhea/malawi-e-commerce-store/internal/orders/public"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/middleware"
)

// aggregateService records what each aggregate handler passed down. Every
// other method is a stub so the fake satisfies public.Service.
type aggregateService struct {
	mineUser   string
	mineFilter public.AggregateFilter
	allUser    string
	allFilter  public.AggregateFilter
}

func (s *aggregateService) Checkout(context.Context, string) (public.Order, error) {
	return public.Order{}, nil
}
func (s *aggregateService) GetByID(context.Context, string, string) (public.Order, error) {
	return public.Order{}, nil
}
func (s *aggregateService) GetByRef(context.Context, string) (public.Order, error) {
	return public.Order{}, nil
}
func (s *aggregateService) ListMine(context.Context, string, public.ListFilter) (public.Page, error) {
	return public.Page{}, nil
}
func (s *aggregateService) Cancel(context.Context, string, string) (public.Order, error) {
	return public.Order{}, nil
}
func (s *aggregateService) MarkPaid(context.Context, string) (public.Order, error) {
	return public.Order{}, nil
}
func (s *aggregateService) ExtendCheckoutHold(context.Context, string) (time.Time, error) {
	return time.Time{}, nil
}
func (s *aggregateService) CancelByRef(context.Context, string) error { return nil }
func (s *aggregateService) SettleExpired(context.Context, string) error {
	return nil
}

func (s *aggregateService) AggregateMine(_ context.Context, userID string, f public.AggregateFilter) (public.Aggregates, error) {
	s.mineUser, s.mineFilter = userID, f
	return sampleAggregates(), nil
}

func (s *aggregateService) AggregateAll(_ context.Context, userID string, f public.AggregateFilter) (public.Aggregates, error) {
	s.allUser, s.allFilter = userID, f
	return sampleAggregates(), nil
}

func sampleAggregates() public.Aggregates {
	return public.Aggregates{
		Orders: public.OrdersAggregate{
			Count: 2,
			ByStatus: []public.StatusTotal{
				{Status: public.StatusPaid, Currency: "USD", Orders: 2, TotalCents: 3000},
			},
			ByCurrency: []public.CurrencyTotal{{Currency: "USD", Orders: 2, TotalCents: 3000}},
		},
		Items: public.ItemsAggregate{
			Lines: 4, UnitsSold: 5,
			RevenueByCurrency: []public.CurrencyRevenue{{Currency: "USD", UnitsSold: 5, RevenueCents: 3000}},
			TopProducts:       []public.ProductTotal{{ProductID: "p1", Currency: "USD", UnitsSold: 5}},
		},
	}
}

func aggregateRouter(svc public.Service) *gin.Engine {
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
	orders.RegisterRoutes(protected, svc)
	return r
}

func TestCustomerAggregatesUsesTokenIdentity(t *testing.T) {
	svc := &aggregateService{}
	r := aggregateRouter(svc)

	// A forged user_id in the query must be ignored.
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/orders/aggregates?user_id=someone-else&status=paid&currency=usd&created_after=2026-01-01", nil)
	req.Header.Set("X-Test-Role", string(userspublic.RoleCustomer))
	req.Header.Set("X-Test-User", "u1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if svc.mineUser != "u1" {
		t.Fatalf("service got user %q, want u1", svc.mineUser)
	}
	if svc.mineFilter.Status != "paid" || svc.mineFilter.Currency != "usd" || svc.mineFilter.CreatedAfter == nil {
		t.Fatalf("filters not forwarded: %+v", svc.mineFilter)
	}
	for _, field := range []string{`"orders"`, `"items"`, `"by_status"`, `"top_products"`, `"revenue_by_currency"`} {
		if !strings.Contains(w.Body.String(), field) {
			t.Errorf("missing %s in response: %s", field, w.Body.String())
		}
	}
}

func TestAdminAggregatesUserFilter(t *testing.T) {
	const customer = "3fa85f64-5717-4562-b3fc-2c963f66afa6"
	svc := &aggregateService{}
	r := aggregateRouter(svc)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/admin/orders/aggregates?user_id="+customer+"&status=paid", nil)
	req.Header.Set("X-Test-Role", string(userspublic.RoleAdmin))
	req.Header.Set("X-Test-User", "admin-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if svc.allUser != customer || svc.allFilter.Status != "paid" {
		t.Fatalf("admin filter not forwarded: %q %+v", svc.allUser, svc.allFilter)
	}
	if svc.mineUser != "" {
		t.Fatalf("admin route called the customer path: %q", svc.mineUser)
	}
}

func TestAdminAggregatesRoleGate(t *testing.T) {
	r := aggregateRouter(&aggregateService{})

	t.Run("customer is forbidden", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/orders/aggregates", nil)
		req.Header.Set("X-Test-Role", string(userspublic.RoleCustomer))
		req.Header.Set("X-Test-User", "u1")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "FORBIDDEN") {
			t.Fatalf("expected 403 FORBIDDEN, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("no identity is unauthorized", func(t *testing.T) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/admin/orders/aggregates", nil))
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestAggregateRoutesAreRegistered(t *testing.T) {
	got := map[string]bool{}
	for _, route := range aggregateRouter(&aggregateService{}).Routes() {
		got[route.Method+" "+route.Path] = true
	}
	for _, want := range []string{
		http.MethodGet + " /api/v1/orders/aggregates",
		http.MethodGet + " /api/v1/admin/orders/aggregates",
	} {
		if !got[want] {
			t.Errorf("route not registered: %s", want)
		}
	}
}

func TestAggregateRejectsBadDate(t *testing.T) {
	r := aggregateRouter(&aggregateService{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/aggregates?created_after=not-a-date", nil)
	req.Header.Set("X-Test-Role", string(userspublic.RoleCustomer))
	req.Header.Set("X-Test-User", "u1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "VALIDATION_ERROR") {
		t.Fatalf("expected 400 VALIDATION_ERROR, got %d: %s", w.Code, w.Body.String())
	}
}
