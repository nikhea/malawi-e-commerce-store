package test

import (
	"context"
	"strings"
	"testing"
	"time"

	authpublic "github.com/nikhea/malawi-e-commerce-store/internal/auth/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/auth/service"
	"github.com/nikhea/malawi-e-commerce-store/internal/auth/utils"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
)

// fakeUsers is an in-memory userspublic.Service.
type fakeUsers struct {
	byID    map[string]record
	byEmail map[string]string
}

type record struct {
	user userspublic.User
	hash string
}

func newFakeUsers() *fakeUsers {
	return &fakeUsers{byID: map[string]record{}, byEmail: map[string]string{}}
}

func (f *fakeUsers) Create(_ context.Context, in userspublic.CreateUserInput) (userspublic.User, error) {
	email := strings.ToLower(in.Email)
	if _, taken := f.byEmail[email]; taken {
		return userspublic.User{}, apperr.Conflict("email already registered")
	}
	u := userspublic.User{ID: "user-" + email, Email: email, Name: in.Name, Role: in.Role}
	f.byID[u.ID] = record{user: u, hash: in.PasswordHash}
	f.byEmail[email] = u.ID
	return u, nil
}

func (f *fakeUsers) GetByID(_ context.Context, id string) (userspublic.User, error) {
	r, ok := f.byID[id]
	if !ok {
		return userspublic.User{}, apperr.NotFound("user not found")
	}
	return r.user, nil
}

func (f *fakeUsers) GetByEmail(_ context.Context, email string) (userspublic.User, error) {
	id, ok := f.byEmail[strings.ToLower(email)]
	if !ok {
		return userspublic.User{}, apperr.NotFound("user not found")
	}
	return f.byID[id].user, nil
}

func (f *fakeUsers) GetCredentials(_ context.Context, email string) (userspublic.Credentials, error) {
	id, ok := f.byEmail[strings.ToLower(email)]
	if !ok {
		return userspublic.Credentials{}, apperr.NotFound("user not found")
	}
	r := f.byID[id]
	return userspublic.Credentials{UserID: r.user.ID, Email: r.user.Email, Role: r.user.Role, PasswordHash: r.hash}, nil
}

func (f *fakeUsers) SetRole(_ context.Context, id string, role userspublic.Role) (userspublic.User, error) {
	r, ok := f.byID[id]
	if !ok {
		return userspublic.User{}, apperr.NotFound("user not found")
	}
	r.user.Role = role
	f.byID[id] = r
	return r.user, nil
}

func testService(users userspublic.Service) authpublic.Service {
	return service.NewService(users, service.Config{
		Secret:      "test-secret",
		TTL:         time.Hour,
		AdminEmails: map[string]struct{}{"boss@malawi.mw": {}},
	})
}

func TestPasswordRoundtrip(t *testing.T) {
	h, err := utils.Hash("hunter2-secure")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if h == "hunter2-secure" {
		t.Fatal("password stored raw")
	}
	if !utils.Verify(h, "hunter2-secure") {
		t.Fatal("valid password rejected")
	}
	if utils.Verify(h, "wrong") {
		t.Fatal("wrong password accepted")
	}
}

func TestJWTEnforcement(t *testing.T) {
	svc := testService(newFakeUsers())
	pair, err := svc.Register(context.Background(), authpublic.RegisterInput{
		Email: "a@malawi.mw", Name: "A", Password: "password1",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if _, err := svc.Parse(pair.Token); err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	for name, tok := range map[string]string{
		"garbage":  "not-a-token",
		"tampered": pair.Token[:len(pair.Token)-2] + "xx",
		"empty":    "",
	} {
		if _, err := svc.Parse(tok); apperr.CodeOf(err) != apperr.CodeUnauthorized {
			t.Fatalf("%s: expected UNAUTHORIZED, got %v", name, err)
		}
	}
}

func TestRegister(t *testing.T) {
	ctx := context.Background()

	t.Run("customer default", func(t *testing.T) {
		pair, err := testService(newFakeUsers()).Register(ctx, authpublic.RegisterInput{
			Email: "shop@malawi.mw", Name: "S", Password: "password1",
		})
		if err != nil {
			t.Fatalf("register: %v", err)
		}
		if pair.Role != string(userspublic.RoleCustomer) || pair.Token == "" {
			t.Fatalf("bad pair: %+v", pair)
		}
	})

	t.Run("admin bootstrap email", func(t *testing.T) {
		pair, err := testService(newFakeUsers()).Register(ctx, authpublic.RegisterInput{
			Email: "boss@malawi.mw", Name: "Boss", Password: "password1",
		})
		if err != nil {
			t.Fatalf("register: %v", err)
		}
		if pair.Role != string(userspublic.RoleAdmin) {
			t.Fatalf("expected admin, got %s", pair.Role)
		}
	})

	t.Run("short password", func(t *testing.T) {
		_, err := testService(newFakeUsers()).Register(ctx, authpublic.RegisterInput{
			Email: "a@malawi.mw", Password: "short",
		})
		if apperr.CodeOf(err) != apperr.CodeValidation {
			t.Fatalf("expected VALIDATION_ERROR, got %v", err)
		}
	})

	t.Run("duplicate", func(t *testing.T) {
		svc := testService(newFakeUsers())
		in := authpublic.RegisterInput{Email: "dup@malawi.mw", Password: "password1"}
		if _, err := svc.Register(ctx, in); err != nil {
			t.Fatalf("first register: %v", err)
		}
		if _, err := svc.Register(ctx, in); apperr.CodeOf(err) != apperr.CodeConflict {
			t.Fatalf("expected CONFLICT, got %v", err)
		}
	})
}

func TestLogin(t *testing.T) {
	ctx := context.Background()
	svc := testService(newFakeUsers())
	if _, err := svc.Register(ctx, authpublic.RegisterInput{
		Email: "lu@malawi.mw", Name: "Lu", Password: "password1",
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	if _, err := svc.Login(ctx, "lu@malawi.mw", "password1"); err != nil {
		t.Fatalf("valid login rejected: %v", err)
	}
	for name, tc := range map[string][2]string{
		"wrong password": {"lu@malawi.mw", "nope-nope-nope"},
		"unknown email":  {"ghost@malawi.mw", "password1"},
	} {
		if _, err := svc.Login(ctx, tc[0], tc[1]); apperr.CodeOf(err) != apperr.CodeUnauthorized {
			t.Fatalf("%s: expected UNAUTHORIZED, got %v", name, err)
		}
	}
}
