package service

import (
	"context"
	"net/mail"
	"strings"
	"time"

	authpublic "github.com/nikhea/malawi-e-commerce-store/internal/auth/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/auth/utils"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
)

var _ authpublic.Service = (*service)(nil)

// Config carries auth's tunables. Assembled in main from config.Config so
// this package never reads env itself (env stays in config/, testable).
type Config struct {
	Secret      string
	TTL         time.Duration
	AdminEmails map[string]struct{} // lowercased owner emails → admin role
}

type service struct {
	users  userspublic.Service
	secret string
	ttl    time.Duration
	admins map[string]struct{}
}

func NewService(users userspublic.Service, cfg Config) authpublic.Service {
	return &service{users: users, secret: cfg.Secret, ttl: cfg.TTL, admins: cfg.AdminEmails}
}

func (s *service) roleFor(email string) userspublic.Role {
	if _, ok := s.admins[email]; ok {
		return userspublic.RoleAdmin
	}
	return userspublic.RoleCustomer
}

func (s *service) Register(ctx context.Context, in authpublic.RegisterInput) (authpublic.TokenPair, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if _, err := mail.ParseAddress(email); err != nil {
		return authpublic.TokenPair{}, apperr.Validation("invalid email address")
	}
	if len(in.Password) < 8 {
		return authpublic.TokenPair{}, apperr.Validation("password must be at least 8 characters")
	}

	hash, err := utils.Hash(in.Password)
	if err != nil {
		return authpublic.TokenPair{}, apperr.Internal(err)
	}

	u, err := s.users.Create(ctx, userspublic.CreateUserInput{
		Email:        email,
		Name:         strings.TrimSpace(in.Name),
		PasswordHash: hash,
		Role:         s.roleFor(email),
	})
	if err != nil {
		return authpublic.TokenPair{}, err // CONFLICT passes through untouched
	}

	return s.issue(u)
}

func (s *service) Login(ctx context.Context, email, password string) (authpublic.TokenPair, error) {
	creds, err := s.users.GetCredentials(ctx, email)
	if err != nil {
		// Deliberately vague: don't reveal whether the email exists.
		return authpublic.TokenPair{}, apperr.Unauthorized("invalid credentials")
	}
	if !utils.Verify(creds.PasswordHash, password) {
		return authpublic.TokenPair{}, apperr.Unauthorized("invalid credentials")
	}

	u, err := s.users.GetByID(ctx, creds.UserID)
	if err != nil {
		return authpublic.TokenPair{}, err
	}
	return s.issue(u)
}

func (s *service) Parse(token string) (authpublic.Claims, error) {
	claims, err := utils.Parse(token, s.secret)
	if err != nil {
		return authpublic.Claims{}, apperr.Unauthorized("invalid token")
	}
	return claims, nil
}

func (s *service) issue(u userspublic.User) (authpublic.TokenPair, error) {
	token, exp, err := utils.Sign(u.ID, u.Email, u.Role, s.secret, s.ttl)
	if err != nil {
		return authpublic.TokenPair{}, apperr.Internal(err)
	}
	return authpublic.TokenPair{
		Token:     token,
		ExpiresAt: exp,
		UserID:    u.ID,
		Email:     u.Email,
		Name:      u.Name,
		Role:      string(u.Role),
	}, nil
}
