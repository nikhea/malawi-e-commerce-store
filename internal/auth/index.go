package auth

import (
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nikhea/malawi-e-commerce-store/internal/auth/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/auth/repository"
	authservice "github.com/nikhea/malawi-e-commerce-store/internal/auth/service"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
)

// Wire builds the auth module around users (accounts), its token store,
// and a mailer for OTP/reset delivery (production: *notify.Service).
// This file is the module's single entry point.
func Wire(pool *pgxpool.Pool, usersSvc userspublic.Service, mailer authservice.Mailer, secret string, ttl time.Duration, adminEmails map[string]struct{}) public.Service {
	return authservice.NewService(usersSvc, repository.NewPostgres(pool), mailer, authservice.Config{
		Secret:      secret,
		TTL:         ttl,
		AdminEmails: adminEmails,
	})
}
