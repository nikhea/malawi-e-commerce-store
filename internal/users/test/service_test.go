package test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nikhea/malawi-e-commerce-store/internal/users/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/users/service"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
)

// fakeRepo is an in-memory Repository. No real DB.
type fakeRepo struct {
	byID    map[string]model.User
	byEmail map[string]string // email → id
}

func newFakeRepo(seed ...model.User) *fakeRepo {
	f := &fakeRepo{byID: map[string]model.User{}, byEmail: map[string]string{}}
	for _, u := range seed {
		f.byID[u.ID] = u
		f.byEmail[u.Email] = u.ID
	}
	return f
}

func (f *fakeRepo) Create(_ context.Context, u model.User) (model.User, error) {
	if _, taken := f.byEmail[strings.ToLower(u.Email)]; taken {
		return model.User{}, apperr.Conflict("email already registered")
	}
	u.ID = "user-" + strings.ToLower(u.Email)
	u.CreatedAt = time.Now()
	u.UpdatedAt = u.CreatedAt
	f.byID[u.ID] = u
	f.byEmail[strings.ToLower(u.Email)] = u.ID
	return u, nil
}

func (f *fakeRepo) GetByID(_ context.Context, id string) (model.User, error) {
	u, ok := f.byID[id]
	if !ok {
		return model.User{}, apperr.NotFound("user not found")
	}
	return u, nil
}

func (f *fakeRepo) GetByEmail(_ context.Context, email string) (model.User, error) {
	id, ok := f.byEmail[strings.ToLower(email)]
	if !ok {
		return model.User{}, apperr.NotFound("user not found")
	}
	return f.byID[id], nil
}

func (f *fakeRepo) SetRole(_ context.Context, id, role string) (model.User, error) {
	u, ok := f.byID[id]
	if !ok {
		return model.User{}, apperr.NotFound("user not found")
	}
	u.Role = role
	f.byID[id] = u
	return u, nil
}

func (f *fakeRepo) SetEmailVerified(_ context.Context, id string, verified bool) (model.User, error) {
	u, ok := f.byID[id]
	if !ok {
		return model.User{}, apperr.NotFound("user not found")
	}
	u.EmailVerified = verified
	f.byID[id] = u
	return u, nil
}

func (f *fakeRepo) SetPasswordHash(_ context.Context, id, hash string) (model.User, error) {
	u, ok := f.byID[id]
	if !ok {
		return model.User{}, apperr.NotFound("user not found")
	}
	u.PasswordHash = hash
	f.byID[id] = u
	return u, nil
}

func TestCreate(t *testing.T) {
	tests := []struct {
		name     string
		seed     []model.User
		input    public.CreateUserInput
		wantRole public.Role
		wantCode apperr.Code // empty = success
	}{
		{
			name:     "defaults to customer",
			input:    public.CreateUserInput{Email: "shop@malawi.mw", Name: "Shopper", PasswordHash: "hash"},
			wantRole: public.RoleCustomer,
		},
		{
			name:     "explicit admin role kept",
			input:    public.CreateUserInput{Email: "boss@malawi.mw", Name: "Boss", PasswordHash: "hash", Role: public.RoleAdmin},
			wantRole: public.RoleAdmin,
		},
		{
			name:     "email lowercased",
			input:    public.CreateUserInput{Email: "SHOP@MALAWI.MW", Name: "S", PasswordHash: "hash"},
			wantRole: public.RoleCustomer,
		},
		{
			name:     "invalid email",
			input:    public.CreateUserInput{Email: "not-an-email", PasswordHash: "hash"},
			wantCode: apperr.CodeValidation,
		},
		{
			name:     "missing password hash",
			input:    public.CreateUserInput{Email: "a@b.mw"},
			wantCode: apperr.CodeValidation,
		},
		{
			name:     "invalid role",
			input:    public.CreateUserInput{Email: "a@b.mw", PasswordHash: "hash", Role: "seller"},
			wantCode: apperr.CodeValidation,
		},
		{
			name:     "duplicate email",
			seed:     []model.User{{ID: "u1", Email: "dup@malawi.mw"}},
			input:    public.CreateUserInput{Email: "dup@malawi.mw", PasswordHash: "hash"},
			wantCode: apperr.CodeConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := service.NewService(newFakeRepo(tt.seed...))
			got, err := svc.Create(context.Background(), tt.input)
			if tt.wantCode != "" {
				if err == nil {
					t.Fatalf("expected error code %s, got nil", tt.wantCode)
				}
				if apperr.CodeOf(err) != tt.wantCode {
					t.Fatalf("expected code %s, got %s (%v)", tt.wantCode, apperr.CodeOf(err), err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Role != tt.wantRole {
				t.Fatalf("expected role %s, got %s", tt.wantRole, got.Role)
			}
			if got.Email != strings.ToLower(strings.TrimSpace(tt.input.Email)) {
				t.Fatalf("expected lowercased email, got %s", got.Email)
			}
		})
	}
}

func TestGetAndSetRole(t *testing.T) {
	seed := model.User{ID: "u1", Email: "a@malawi.mw", Name: "A", Role: "customer"}
	svc := service.NewService(newFakeRepo(seed))
	ctx := context.Background()

	if _, err := svc.GetByID(ctx, "missing"); apperr.CodeOf(err) != apperr.CodeNotFound {
		t.Fatalf("expected NOT_FOUND, got %v", err)
	}
	if _, err := svc.GetByID(ctx, ""); apperr.CodeOf(err) != apperr.CodeValidation {
		t.Fatalf("expected VALIDATION_ERROR for empty id, got %v", err)
	}
	got, err := svc.GetByEmail(ctx, "A@MALAWI.MW")
	if err != nil || got.ID != "u1" {
		t.Fatalf("expected case-insensitive lookup, got %v %v", got, err)
	}
	promoted, err := svc.SetRole(ctx, "u1", public.RoleAdmin)
	if err != nil || promoted.Role != public.RoleAdmin {
		t.Fatalf("expected admin promotion, got %v %v", promoted, err)
	}
	if _, err := svc.SetRole(ctx, "u1", "seller"); apperr.CodeOf(err) != apperr.CodeValidation {
		t.Fatalf("expected VALIDATION_ERROR for bad role, got %v", err)
	}
	if _, err := svc.SetRole(ctx, "missing", public.RoleAdmin); apperr.CodeOf(err) != apperr.CodeNotFound {
		t.Fatalf("expected NOT_FOUND, got %v", err)
	}
}
