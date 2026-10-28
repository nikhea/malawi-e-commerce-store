package test

import (
	"context"
	"sort"
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

func (f *fakeRepo) List(_ context.Context, filter model.ListFilter) ([]model.User, int64, error) {
	q := strings.ToLower(filter.Query)
	out := make([]model.User, 0, len(f.byID))
	for _, u := range f.byID {
		if q != "" && !strings.Contains(strings.ToLower(u.Email), q) &&
			!strings.Contains(strings.ToLower(u.Name), q) {
			continue
		}
		if filter.Role != "" && u.Role != filter.Role {
			continue
		}
		if filter.EmailVerified != nil && u.EmailVerified != *filter.EmailVerified {
			continue
		}
		if filter.CreatedAfter != nil && u.CreatedAt.Before(*filter.CreatedAfter) {
			continue
		}
		if filter.CreatedBefore != nil && u.CreatedAt.After(*filter.CreatedBefore) {
			continue
		}
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool {
		switch filter.Sort {
		case "oldest":
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		case "name_asc":
			return out[i].Name < out[j].Name
		case "name_desc":
			return out[i].Name > out[j].Name
		case "email_asc":
			return out[i].Email < out[j].Email
		case "email_desc":
			return out[i].Email > out[j].Email
		default:
			return out[i].CreatedAt.After(out[j].CreatedAt)
		}
	})
	total := int64(len(out))
	start := filter.Offset
	if start > len(out) {
		start = len(out)
	}
	end := start + filter.Limit
	if end > len(out) {
		end = len(out)
	}
	return out[start:end], total, nil
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

func TestList(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	seed := []model.User{
		{ID: "u1", Email: "aisha@malawi.mw", Name: "Aisha Banda", Role: "customer", EmailVerified: true, CreatedAt: now.Add(-3 * time.Hour)},
		{ID: "u2", Email: "boss@malawi.mw", Name: "Boss Phiri", Role: "admin", EmailVerified: true, CreatedAt: now.Add(-2 * time.Hour)},
		{ID: "u3", Email: "chisomo@malawi.mw", Name: "Chisomo Zulu", Role: "customer", EmailVerified: false, CreatedAt: now.Add(-time.Hour)},
	}
	svc := service.NewService(newFakeRepo(seed...))

	t.Run("search and role filters", func(t *testing.T) {
		page, err := svc.List(ctx, public.ListFilter{Query: "aisha"})
		if err != nil || page.Meta.Total != 1 || page.Items[0].ID != "u1" {
			t.Fatalf("search failed: %+v %v", page, err)
		}
		page, err = svc.List(ctx, public.ListFilter{Role: public.RoleAdmin})
		if err != nil || page.Meta.Total != 1 || page.Items[0].ID != "u2" {
			t.Fatalf("role filter failed: %+v %v", page, err)
		}
		verified := false
		page, err = svc.List(ctx, public.ListFilter{EmailVerified: &verified})
		if err != nil || page.Meta.Total != 1 || page.Items[0].ID != "u3" {
			t.Fatalf("verification filter failed: %+v %v", page, err)
		}
	})

	t.Run("pagination and sort", func(t *testing.T) {
		page, err := svc.List(ctx, public.ListFilter{Sort: "name_asc", Page: 2, Limit: 2})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if page.Meta.Total != 3 || page.Meta.Page != 2 || len(page.Items) != 1 {
			t.Fatalf("bad page: %+v", page.Meta)
		}
		if page.Items[0].ID != "u3" {
			t.Fatalf("expected third name, got %+v", page.Items[0])
		}
	})

	t.Run("validation", func(t *testing.T) {
		after := now
		before := now.Add(-time.Hour)
		cases := []public.ListFilter{
			{Role: "seller"},
			{Sort: "bogus"},
			{Limit: -1},
			{Offset: -1},
			{Page: -1},
			{CreatedAfter: &after, CreatedBefore: &before},
		}
		for _, f := range cases {
			if _, err := svc.List(ctx, f); apperr.CodeOf(err) != apperr.CodeValidation {
				t.Fatalf("expected VALIDATION_ERROR for %+v, got %v", f, err)
			}
		}
	})
}
