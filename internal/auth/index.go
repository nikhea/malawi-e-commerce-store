package auth

import (
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nikhea/malawi-e-commerce-store/internal/auth/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/auth/repository"
	authservice "github.com/nikhea/malawi-e-commerce-store/internal/auth/service"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/events"
)

// Wire builds the auth module around users (accounts), its own token
// store, and the event bus (OTP/reset mail fan-out). This file is the
// module's single entry point.
func Wire(pool *pgxpool.Pool, usersSvc userspublic.Service, bus *events.Bus, secret string, ttl time.Duration, adminEmails map[string]struct{}) public.Service {
	return authservice.NewService(usersSvc, repository.NewPostgres(pool), bus, authservice.Config{
		Secret:      secret,
		TTL:         ttl,
		AdminEmails: adminEmails,
	})
}
