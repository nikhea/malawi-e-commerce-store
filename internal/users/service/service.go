package service

import (
	"context"
	"net/mail"
	"strings"

	"github.com/nikhea/malawi-e-commerce-store/internal/users/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
)

var _ public.Service = (*service)(nil)

// Repository is the port this service needs from persistence. The pgx
// implementation adapts to it; tests substitute a fake.
type Repository interface {
	Create(ctx context.Context, u model.User) (model.User, error)
	GetByID(ctx context.Context, id string) (model.User, error)
	GetByEmail(ctx context.Context, email string) (model.User, error)
	SetRole(ctx context.Context, id, role string) (model.User, error)
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
		ID:        u.ID,
		Email:     u.Email,
		Name:      u.Name,
		Role:      public.Role(u.Role),
		CreatedAt: u.CreatedAt,
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
