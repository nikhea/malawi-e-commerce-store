package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	authpublic "github.com/nikhea/malawi-e-commerce-store/internal/auth/public"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/middleware"
)

// fakeAuth accepts exactly "good-token" and rejects everything else.
type fakeAuth struct{}

func (fakeAuth) Register(context.Context, authpublic.RegisterInput) (authpublic.TokenPair, error) {
	return authpublic.TokenPair{}, nil
}
func (fakeAuth) Login(context.Context, string, string) (authpublic.TokenPair, error) {
	return authpublic.TokenPair{}, nil
}
func (fakeAuth) Parse(token string) (authpublic.Claims, error) {
	if token != "head.pay.sig" {
		return authpublic.Claims{}, errBadToken
	}
	return authpublic.Claims{UserID: "u1", Role: userspublic.RoleCustomer}, nil
}
func (fakeAuth) RequestVerification(context.Context, string) error   { return nil }
func (fakeAuth) VerifyEmail(context.Context, string, string) error   { return nil }
func (fakeAuth) RequestPasswordReset(context.Context, string) error  { return nil }
func (fakeAuth) ResetPassword(context.Context, string, string) error { return nil }
func (fakeAuth) Refresh(context.Context, string) (authpublic.TokenPair, error) {
	return authpublic.TokenPair{}, nil
}
func (fakeAuth) Logout(context.Context, string) error { return nil }

type badTokenError struct{}

func (badTokenError) Error() string { return "bad token" }

var errBadToken error = badTokenError{}

func jwtTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.JWT(fakeAuth{}))
	r.GET("/me", func(c *gin.Context) {
		c.String(http.StatusOK, "uid=%s", middleware.UserIDOf(c))
	})
	return r
}

func TestJWTBearerAndBare(t *testing.T) {
	r := jwtTestRouter()

	// Canonical "Bearer <token>".
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer head.pay.sig")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || w.Body.String() != "uid=u1" {
		t.Fatalf("bearer: got %d %q", w.Code, w.Body.String())
	}

	// Bare token (Swagger UI's apiKey box sends the raw value).
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "head.pay.sig")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || w.Body.String() != "uid=u1" {
		t.Fatalf("bare: got %d %q", w.Code, w.Body.String())
	}
}

func TestJWTMissingOrGarbage(t *testing.T) {
	r := jwtTestRouter()
	for name, header := range map[string]string{
		"missing":  "",
		"garbage":  "not-a-token",
		"two-part": "abc.def",
		"bad-jwt":  "aaa.bbb.ccc",
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/me", nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("%s: expected 401, got %d", name, w.Code)
		}
	}
}
