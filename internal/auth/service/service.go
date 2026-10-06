package service

import (
	"context"
	"net/mail"
	"strings"
	"time"

	authpublic "github.com/nikhea/malawi-e-commerce-store/internal/auth/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/auth/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/auth/utils"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	"github.com/nikhea/malawi-e-commerce-store/pkg/events"
)

var _ authpublic.Service = (*service)(nil)

// Repository is the port this service needs for token stores (OTP,
// resets, refresh lineage). The pgx implementation adapts to it; tests
// substitute a fake.
type Repository interface {
	UpsertVerification(ctx context.Context, userID, codeHash string, expiresAt time.Time) error
	ConsumeVerification(ctx context.Context, userID, codeHash string) error
	CreateReset(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error
	ConsumeReset(ctx context.Context, tokenHash string) (model.PasswordReset, error)
	CreateRefresh(ctx context.Context, userID, tokenHash string, expiresAt time.Time) (string, error)
	GetRefresh(ctx context.Context, tokenHash string) (model.RefreshToken, error)
	RotateRefresh(ctx context.Context, oldID, newID string) error
	RevokeRefresh(ctx context.Context, tokenHash string) error
	RevokeUserRefresh(ctx context.Context, userID string) error
	RefreshUsedElsewhere(ctx context.Context, tokenHash string) (model.RefreshToken, bool, error)
}

// Config carries auth's tunables. Assembled in main from config.Config so
// this package never reads env itself (env stays in config/, testable).
type Config struct {
	Secret      string
	TTL         time.Duration
	AdminEmails map[string]struct{} // lowercased owner emails → admin role
}

type service struct {
	users  userspublic.Service
	repo   Repository
	bus    *events.Bus
	secret string
	ttl    time.Duration
	admins map[string]struct{}
}

func NewService(users userspublic.Service, repo Repository, bus *events.Bus, cfg Config) authpublic.Service {
	return &service{users: users, repo: repo, bus: bus, secret: cfg.Secret, ttl: cfg.TTL, admins: cfg.AdminEmails}
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
	if len(in.Password) < utils.MinPasswordLength {
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

	// Verification email goes out async (worker); registration answers now.
	if err := s.sendVerification(ctx, u); err != nil {
		return authpublic.TokenPair{}, err
	}
	return s.issue(ctx, u)
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
	return s.issue(ctx, u)
}

func (s *service) Parse(token string) (authpublic.Claims, error) {
	claims, err := utils.Parse(token, s.secret)
	if err != nil {
		return authpublic.Claims{}, apperr.Unauthorized("invalid token")
	}
	return claims, nil
}

// issue mints the access token AND a refresh token (stored hashed).
func (s *service) issue(ctx context.Context, u userspublic.User) (authpublic.TokenPair, error) {
	token, exp, err := utils.Sign(u.ID, u.Email, u.Role, s.secret, s.ttl)
	if err != nil {
		return authpublic.TokenPair{}, apperr.Internal(err)
	}
	refresh, refreshExp, err := s.newRefresh(ctx, u.ID)
	if err != nil {
		return authpublic.TokenPair{}, err
	}
	return authpublic.TokenPair{
		Token: token, ExpiresAt: exp,
		RefreshToken: refresh, RefreshExpires: refreshExp,
		UserID: u.ID, Email: u.Email, Name: u.Name, Role: string(u.Role),
	}, nil
}

func (s *service) newRefresh(ctx context.Context, userID string) (token string, expires time.Time, err error) {
	token, err = utils.Token()
	if err != nil {
		return "", time.Time{}, apperr.Internal(err)
	}
	expires = time.Now().Add(utils.RefreshExpiry)
	if _, err := s.repo.CreateRefresh(ctx, userID, utils.HashSecret(token), expires); err != nil {
		return "", time.Time{}, apperr.Internal(err)
	}
	return token, expires, nil
}

// sendVerification creates an OTP and publishes the mail event. Shared by
// Register and RequestVerification (resend = new code, same path).
func (s *service) sendVerification(ctx context.Context, u userspublic.User) error {
	code, err := utils.OTP()
	if err != nil {
		return apperr.Internal(err)
	}
	if err := s.repo.UpsertVerification(ctx, u.ID, utils.HashSecret(code), time.Now().Add(utils.OTPExpiry)); err != nil {
		return apperr.Internal(err)
	}
	s.bus.Publish(ctx, events.Event{
		Name:    authpublic.EmailVerificationRequested,
		Payload: authpublic.VerificationMail{UserID: u.ID, Email: u.Email, Name: u.Name, Code: code},
	})
	return nil
}

func (s *service) RequestVerification(ctx context.Context, email string) error {
	u, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return nil // no enumeration: unknown emails succeed silently
	}
	if u.EmailVerified {
		return nil // nothing to do
	}
	full, err := s.users.GetByID(ctx, u.ID)
	if err != nil {
		return err
	}
	return s.sendVerification(ctx, full)
}

func (s *service) VerifyEmail(ctx context.Context, email, code string) error {
	u, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return apperr.Unauthorized("invalid code")
	}
	if err := s.repo.ConsumeVerification(ctx, u.ID, utils.HashSecret(strings.TrimSpace(code))); err != nil {
		return err
	}
	_, err = s.users.SetEmailVerified(ctx, u.ID, true)
	return err
}

func (s *service) RequestPasswordReset(ctx context.Context, email string) error {
	u, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return nil // no enumeration
	}
	token, err := utils.Token()
	if err != nil {
		return apperr.Internal(err)
	}
	if err := s.repo.CreateReset(ctx, u.ID, utils.HashSecret(token), time.Now().Add(utils.ResetExpiry)); err != nil {
		return apperr.Internal(err)
	}
	full, err := s.users.GetByID(ctx, u.ID)
	if err != nil {
		return err
	}
	s.bus.Publish(ctx, events.Event{
		Name:    authpublic.PasswordResetRequested,
		Payload: authpublic.ResetMail{UserID: u.ID, Email: u.Email, Name: full.Name, Token: token},
	})
	return nil
}

func (s *service) ResetPassword(ctx context.Context, token, newPassword string) error {
	if len(newPassword) < utils.MinPasswordLength {
		return apperr.Validation("password must be at least 8 characters")
	}
	pr, err := s.repo.ConsumeReset(ctx, utils.HashSecret(strings.TrimSpace(token)))
	if err != nil {
		return err
	}
	hash, err := utils.Hash(newPassword)
	if err != nil {
		return apperr.Internal(err)
	}
	if _, err := s.users.SetPasswordHash(ctx, pr.UserID, hash); err != nil {
		return err
	}
	// Reset via inbox proves ownership: verify the email too, and kill
	// every session (a stolen password dies with the reset).
	if _, err := s.users.SetEmailVerified(ctx, pr.UserID, true); err != nil {
		return err
	}
	return s.repo.RevokeUserRefresh(ctx, pr.UserID)
}

func (s *service) Refresh(ctx context.Context, refreshToken string) (authpublic.TokenPair, error) {
	hash := utils.HashSecret(strings.TrimSpace(refreshToken))
	rt, err := s.repo.GetRefresh(ctx, hash)
	if err != nil {
		// Not live: unknown, expired, or revoked. Reuse of a ROTATED
		// token is theft — kill the whole chain loudly.
		if row, reused, rerr := s.repo.RefreshUsedElsewhere(ctx, hash); rerr == nil && reused {
			_ = s.repo.RevokeUserRefresh(ctx, row.UserID)
			return authpublic.TokenPair{}, apperr.Unauthorized("session compromised, signed out everywhere")
		}
		return authpublic.TokenPair{}, apperr.Unauthorized("invalid refresh token")
	}

	u, err := s.users.GetByID(ctx, rt.UserID)
	if err != nil {
		return authpublic.TokenPair{}, err
	}
	token, exp, err := utils.Sign(u.ID, u.Email, u.Role, s.secret, s.ttl)
	if err != nil {
		return authpublic.TokenPair{}, apperr.Internal(err)
	}
	newRefresh, newExp, err := s.newRefresh(ctx, u.ID)
	if err != nil {
		return authpublic.TokenPair{}, err
	}
	// Rotate AFTER the replacement exists: crash between = two live
	// tokens (tolerable); reverse order risks locking the user out.
	newID, err := s.refreshID(ctx, newRefresh)
	if err != nil {
		return authpublic.TokenPair{}, err
	}
	if err := s.repo.RotateRefresh(ctx, rt.ID, newID); err != nil {
		return authpublic.TokenPair{}, err
	}
	return authpublic.TokenPair{
		Token: token, ExpiresAt: exp,
		RefreshToken: newRefresh, RefreshExpires: newExp,
		UserID: u.ID, Email: u.Email, Name: u.Name, Role: string(u.Role),
	}, nil
}

// refreshID resolves a just-created token to its row id for rotation
// linking. (The hash is unique, so this read is exact.)
func (s *service) refreshID(ctx context.Context, refreshToken string) (string, error) {
	rt, err := s.repo.GetRefresh(ctx, utils.HashSecret(refreshToken))
	if err != nil {
		return "", err
	}
	return rt.ID, nil
}

func (s *service) Logout(ctx context.Context, refreshToken string) error {
	if strings.TrimSpace(refreshToken) == "" {
		return nil
	}
	_ = s.repo.RevokeRefresh(ctx, utils.HashSecret(refreshToken))
	return nil // unknown tokens succeed silently
}
