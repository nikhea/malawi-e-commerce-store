package service

import (
	"context"
	"net/mail"
	"strings"

	"github.com/nikhea/malawi-e-commerce-store/internal/users/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	"github.com/nikhea/malawi-e-commerce-store/pkg/pagination"
)

var _ public.Service = (*service)(nil)

// Repository is the port this service needs from persistence. The pgx
// implementation adapts to it; tests substitute a fake.
type Repository interface {
	Create(ctx context.Context, u model.User) (model.User, error)
	GetByID(ctx context.Context, id string) (model.User, error)
	GetByEmail(ctx context.Context, email string) (model.User, error)
	List(ctx context.Context, f model.ListFilter) ([]model.User, int64, error)
	SetRole(ctx context.Context, id, role string) (model.User, error)
	SetEmailVerified(ctx context.Context, id string, verified bool) (model.User, error)
	SetPasswordHash(ctx context.Context, id, hash string) (model.User, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) public.Service {
	return &service{repo: repo}
}

func validRole(r public.Role) bool {
	return r == public.RoleAdmin || r == public.RoleCustomer
}

func toPublic(u model.User) public.User {
	return public.User{
		ID:            u.ID,
		Email:         u.Email,
		Name:          u.Name,
		Role:          public.Role(u.Role),
		EmailVerified: u.EmailVerified,
		CreatedAt:     u.CreatedAt,
	}
}

func (s *service) Create(ctx context.Context, in public.CreateUserInput) (public.User, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if _, err := mail.ParseAddress(email); err != nil {
		return public.User{}, apperr.Validation("invalid email address")
	}
	if in.PasswordHash == "" {
		return public.User{}, apperr.Validation("password hash is required")
	}
	role := in.Role
	if role == "" {
		role = public.RoleCustomer
	}
	if !validRole(role) {
		return public.User{}, apperr.Validation("invalid role")
	}

	u, err := s.repo.Create(ctx, model.User{
		Email:        email,
		PasswordHash: in.PasswordHash,
		Name:         strings.TrimSpace(in.Name),
		Role:         string(role),
	})
	if err != nil {
		return public.User{}, err // repository returns coded errors
	}
	return toPublic(u), nil
}

func (s *service) GetByID(ctx context.Context, id string) (public.User, error) {
	if strings.TrimSpace(id) == "" {
		return public.User{}, apperr.Validation("id is required")
	}
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return public.User{}, err
	}
	return toPublic(u), nil
}

func (s *service) GetByEmail(ctx context.Context, email string) (public.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return public.User{}, apperr.Validation("email is required")
	}
	u, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		return public.User{}, err
	}
	return toPublic(u), nil
}

var userSorts = map[string]bool{
	"newest":     true,
	"oldest":     true,
	"name_asc":   true,
	"name_desc":  true,
	"email_asc":  true,
	"email_desc": true,
}

func (s *service) List(ctx context.Context, f public.ListFilter) (public.Page, error) {
	if f.Limit < 0 {
		return public.Page{}, apperr.Validation("limit cannot be negative")
	}
	if f.Offset < 0 {
		return public.Page{}, apperr.Validation("offset cannot be negative")
	}
	if f.Page < 0 {
		return public.Page{}, apperr.Validation("page cannot be negative")
	}
	if f.Role != "" && !validRole(f.Role) {
		return public.Page{}, apperr.Validation("role must be admin or customer")
	}
	if f.CreatedAfter != nil && f.CreatedBefore != nil && f.CreatedAfter.After(*f.CreatedBefore) {
		return public.Page{}, apperr.Validation("created_after cannot be after created_before")
	}

	sortKey := strings.TrimSpace(f.Sort)
	if sortKey == "" {
		sortKey = "newest"
	}
	if !userSorts[sortKey] {
		return public.Page{}, apperr.Validation("unknown sort; use newest, oldest, name_asc, name_desc, email_asc or email_desc")
	}

	limit, offset := pagination.Normalize(f.Page, f.Limit, f.Offset)
	users, total, err := s.repo.List(ctx, model.ListFilter{
		Query: strings.TrimSpace(f.Query), Role: string(f.Role),
		EmailVerified: f.EmailVerified, CreatedAfter: f.CreatedAfter,
		CreatedBefore: f.CreatedBefore, Sort: sortKey, Limit: limit, Offset: offset,
	})
	if err != nil {
		return public.Page{}, err
	}
	out := make([]public.User, 0, len(users))
	for _, u := range users {
		out = append(out, toPublic(u))
	}
	return public.Page{Items: out, Meta: pagination.NewMeta(total, limit, offset)}, nil
}

func (s *service) GetCredentials(ctx context.Context, email string) (public.Credentials, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return public.Credentials{}, apperr.Validation("email is required")
	}
	u, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		return public.Credentials{}, err
	}
	return public.Credentials{
		UserID:       u.ID,
		Email:        u.Email,
		Role:         public.Role(u.Role),
		PasswordHash: u.PasswordHash,
	}, nil
}

func (s *service) SetRole(ctx context.Context, id string, role public.Role) (public.User, error) {
	if strings.TrimSpace(id) == "" {
		return public.User{}, apperr.Validation("id is required")
	}
	if !validRole(role) {
		return public.User{}, apperr.Validation("invalid role")
	}
	u, err := s.repo.SetRole(ctx, id, string(role))
	if err != nil {
		return public.User{}, err
	}
	return toPublic(u), nil
}

func (s *service) SetEmailVerified(ctx context.Context, id string, verified bool) (public.User, error) {
	if strings.TrimSpace(id) == "" {
		return public.User{}, apperr.Validation("id is required")
	}
	u, err := s.repo.SetEmailVerified(ctx, id, verified)
	if err != nil {
		return public.User{}, err
	}
	return toPublic(u), nil
}

func (s *service) SetPasswordHash(ctx context.Context, id, hash string) (public.User, error) {
	if strings.TrimSpace(id) == "" {
		return public.User{}, apperr.Validation("id is required")
	}
	if hash == "" {
		return public.User{}, apperr.Validation("password hash is required")
	}
	u, err := s.repo.SetPasswordHash(ctx, id, hash)
	if err != nil {
		return public.User{}, err
	}
	return toPublic(u), nil
}
