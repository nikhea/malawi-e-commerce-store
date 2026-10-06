package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/config"
	"github.com/nikhea/malawi-e-commerce-store/db"
	users "github.com/nikhea/malawi-e-commerce-store/internal/users"
	"github.com/nikhea/malawi-e-commerce-store/internal/users/repository"
	userservice "github.com/nikhea/malawi-e-commerce-store/internal/users/service"
	"github.com/nikhea/malawi-e-commerce-store/pkg/middleware"
)

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

	r := gin.New()
	r.Use(gin.Recovery(), middleware.RequestID(), middleware.Logger())

	// Infra check: raw {"status":"ok"}, no response envelope.
	// Versioned module APIs (/api/v1/…) use pkg/response instead.
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	users.RegisterRoutes(r.Group("/api/v1"), usersSvc)

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
