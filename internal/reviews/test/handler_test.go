package test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/internal/reviews"
	"github.com/nikhea/malawi-e-commerce-store/internal/reviews/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/pagination"
)

// fakeService records the caller identity the handler resolved.
type fakeService struct {
	lastUserID string
	lastFilter public.ListFilter
}

func (f *fakeService) Create(context.Context, public.CreateReviewInput) (public.Review, error) {
	return public.Review{}, nil
}

func (f *fakeService) UpdateMine(context.Context, string, string, public.UpdateReviewInput) (public.Review, error) {
	return public.Review{}, nil
}

func (f *fakeService) DeleteMine(context.Context, string, string) error { return nil }

func (f *fakeService) DeleteAny(context.Context, string) error { return nil }

func (f *fakeService) Summary(context.Context, string, public.ListFilter) (public.Summary, error) {
	return public.Summary{}, nil
}

func (f *fakeService) ListMine(_ context.Context, userID string, filter public.ListFilter) (public.MyReviewsPage, error) {
	f.lastUserID = userID
	f.lastFilter = filter
	return public.MyReviewsPage{
		Items: []public.MyReview{{
			ID: "rev-1", UserID: userID, ProductID: "p1", Rating: 5, Title: "Great",
			Product: public.MyReviewProduct{ID: "p1", Name: "Oil", Slug: "oil"},
		}},
		Meta: pagination.NewMeta(1, 12, 0),
	}, nil
}

func router(svc public.Service) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	open := r.Group("/api/v1")
	protected := r.Group("/api/v1")
	protected.Use(func(c *gin.Context) {
		if user := c.GetHeader("X-Test-User"); user != "" {
			c.Set("user_id", user)
		}
		c.Next()
	})
	reviews.RegisterRoutes(open, protected, svc)
	return r
}

func TestListMyReviewsUsesTokenIdentity(t *testing.T) {
	svc := &fakeService{}
	r := router(svc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/reviews/me?sort=highest&page=2&limit=5", nil)
	req.Header.Set("X-Test-User", "u1")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if svc.lastUserID != "u1" {
		t.Fatalf("service got user %q, want u1", svc.lastUserID)
	}
	if svc.lastFilter.Sort != "highest" || svc.lastFilter.Page != 2 || svc.lastFilter.Limit != 5 {
		t.Fatalf("query not forwarded: %+v", svc.lastFilter)
	}
	if !strings.Contains(w.Body.String(), `"meta"`) || !strings.Contains(w.Body.String(), `"rev-1"`) {
		t.Fatalf("missing paged envelope: %s", w.Body.String())
	}
}

// /reviews/me is a static segment that must not be captured by /reviews/{id}
// (the id routes are PATCH/DELETE, but registration order still matters).
func TestMyReviewsRouteIsStatic(t *testing.T) {
	r := router(&fakeService{})
	for _, route := range r.Routes() {
		if route.Method == http.MethodGet && route.Path == "/api/v1/reviews/me" {
			return
		}
	}
	t.Fatal("GET /api/v1/reviews/me is not registered")
}
