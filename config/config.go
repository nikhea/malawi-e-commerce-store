package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config is the single typed env loader shared by cmd/api and cmd/worker.
// Local dev loads .env through Air (.air.toml env_files); in other
// environments the variables must be exported before the binary runs.
type Config struct {
	AppPort string
	AppURL  string

	// Env is "development" or "production". Production switches Gin to
	// release mode (no debug logs/routes) — see wire.go newRouter.
	Env string

	DatabaseURL string

	RedisURL      string
	RedisHost     string
	RedisPort     int
	RedisPassword string
	RedisDB       int

	JWTSecret   string
	JWTTTLHours int

	// AdminEmails bootstraps the first admins (chicken-and-egg fix: the
	// first admin cannot be created via admin-only API). Auth's register
	// assigns the admin role to these addresses. Comma-separated.
	AdminEmails []string

	EmailService  string
	EmailAddress  string
	EmailPassword string

	CloudinaryURL          string
	CloudName              string
	CloudAPIKey            string
	CloudAPISecret         string
	CloudinaryUploadPreset string

	StripeSecretKey      string
	StripePublishableKey string
	StripeWebhookSecret  string
	StripePricePro       string
	StripePriceScale     string

	QRSigningSecret string
}

// Load reads the environment into Config. DATABASE_URL and JWT_SECRET are
// required; everything else falls back to local-dev defaults. It returns an
// error instead of panicking so callers decide how to fail (log.Fatal in
// main, t.Skip in tests).
func Load() (Config, error) {
	cfg := Config{
		AppPort:                envOr("APP_PORT", "8080"),
		AppURL:                 envOr("APP_URL", "http://localhost:8080"),
		Env:                    envOr("APP_ENV", "development"),
		DatabaseURL:            os.Getenv("DATABASE_URL"),
		RedisURL:               envOr("REDIS_URL", "redis://localhost:6379/0"),
		RedisHost:              envOr("REDIS_HOST", "localhost"),
		RedisPassword:          os.Getenv("REDIS_PASSWORD"),
		JWTSecret:              os.Getenv("JWT_SECRET"),
		EmailService:           envOr("EMAIL_SERVICE", "Gmail"),
		EmailAddress:           os.Getenv("EMAIL_ADDRESS"),
		EmailPassword:          os.Getenv("EMAIL_PASSWORD"),
		CloudinaryURL:          os.Getenv("CLOUDINARY_URL"),
		CloudName:              os.Getenv("CLOUD_NAME"),
		CloudAPIKey:            os.Getenv("CLOUD_API_KEY"),
		CloudAPISecret:         os.Getenv("CLOUD_API_SECRET"),
		CloudinaryUploadPreset: os.Getenv("CLOUDINARY_UPLOAD_PRESET"),
		StripeSecretKey:        os.Getenv("STRIPE_SECRET_KEY"),
		StripePublishableKey:   os.Getenv("NEXT_PUBLIC_STRIPE_PUBLISHABLE_KEY"),
		StripeWebhookSecret:    os.Getenv("STRIPE_WEBHOOKS_SIGNING_SECRET"),
		StripePricePro:         os.Getenv("STRIPE_PRICE_PRO"),
		StripePriceScale:       os.Getenv("STRIPE_PRICE_SCALE"),
		QRSigningSecret:        os.Getenv("QR_SIGNING_SECRET"),
	}

	var err error
	if cfg.RedisPort, err = envIntOr("REDIS_PORT", 6379); err != nil {
		return Config{}, fmt.Errorf("invalid REDIS_PORT: %w", err)
	}
	if cfg.RedisDB, err = envIntOr("REDIS_DB", 0); err != nil {
		return Config{}, fmt.Errorf("invalid REDIS_DB: %w", err)
	}
	if cfg.JWTTTLHours, err = envIntOr("JWT_TTL_HOURS", 24); err != nil {
		return Config{}, fmt.Errorf("invalid JWT_TTL_HOURS: %w", err)
	}
	cfg.AdminEmails = envList("ADMIN_EMAILS")

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return Config{}, errors.New("JWT_SECRET is required")
	}

	return cfg, nil
}

// Addr returns the listen address derived from APP_PORT (e.g. ":8080").
func (c Config) Addr() string {
	return ":" + c.AppPort
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOr(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	return strconv.Atoi(v)
}

// envList parses a comma-separated env var, trimming spaces and dropping
// empties. Unset → nil.
func envList(key string) []string {
	raw := os.Getenv(key)
	if raw == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
