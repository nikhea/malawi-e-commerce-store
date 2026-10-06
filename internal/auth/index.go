package auth

import (
	"time"

	"github.com/nikhea/malawi-e-commerce-store/internal/auth/public"
	authservice "github.com/nikhea/malawi-e-commerce-store/internal/auth/service"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
)

// Wire builds the auth module around an already-built users service.
// This file is the module's single entry point — cmd/* calls Wire
// instead of assembling the layers by hand.
func Wire(usersSvc userspublic.Service, secret string, ttl time.Duration, adminEmails map[string]struct{}) public.Service {
	return authservice.NewService(usersSvc, authservice.Config{
		Secret:      secret,
		TTL:         ttl,
		AdminEmails: adminEmails,
	})
}
