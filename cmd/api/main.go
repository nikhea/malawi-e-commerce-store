package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/config"
	"github.com/nikhea/malawi-e-commerce-store/db"
	_ "github.com/nikhea/malawi-e-commerce-store/docs"
	auth "github.com/nikhea/malawi-e-commerce-store/internal/auth"
	authservice "github.com/nikhea/malawi-e-commerce-store/internal/auth/service"
	users "github.com/nikhea/malawi-e-commerce-store/internal/users"
	"github.com/nikhea/malawi-e-commerce-store/internal/users/repository"
	userservice "github.com/nikhea/malawi-e-commerce-store/internal/users/service"
	"github.com/nikhea/malawi-e-commerce-store/pkg/middleware"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title Malawi E-Commerce Store API
// @version 1.0
// @description Single-storefront e-commerce API (users, catalog, cart, orders, payments).
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT. Prefix with "Bearer ".

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer pool.Close()

	usersSvc := userservice.NewService(repository.NewPostgres(pool))

	admins := make(map[string]struct{}, len(cfg.AdminEmails))
	for _, e := range cfg.AdminEmails {
		// Lowercased: service looks up the already-lowercased email.
		admins[strings.ToLower(strings.TrimSpace(e))] = struct{}{}
	}
	authSvc := authservice.NewService(usersSvc, authservice.Config{
		Secret:      cfg.JWTSecret,
		TTL:         time.Duration(cfg.JWTTTLHours) * time.Hour,
		AdminEmails: admins,
	})

	r := gin.New()
	r.Use(gin.Recovery(), middleware.RequestID(), middleware.Logger())

	// Infra check: raw {"status":"ok"}, no response envelope.
	// Versioned module APIs (/api/v1/…) use pkg/response instead.
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Open: register/login must NOT sit behind JWT.
	open := r.Group("/api/v1")
	auth.RegisterRoutes(open, authSvc)

	// Protected: JWT runs first (sets role + user id), then module routes.
	// RequireRole inside users' admin group now enforces for real.
	protected := r.Group("/api/v1")
	protected.Use(middleware.JWT(authSvc))
	users.RegisterRoutes(protected, usersSvc)

	// Generated API docs (docs/ is committed; refresh with
	// `swag init -g cmd/api/main.go --parseInternal -o docs`).
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))

	srv := &http.Server{Addr: cfg.Addr(), Handler: r}

	go func() {
		log.Printf("listening on %s", cfg.Addr())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("serve: %v", err)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
	log.Println("stopped")
}
