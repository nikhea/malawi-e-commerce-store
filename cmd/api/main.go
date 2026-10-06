package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/nikhea/malawi-e-commerce-store/config"
	"github.com/nikhea/malawi-e-commerce-store/db"
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

	// All module + infra assembly lives in wire.go.
	a, err := wireApp(ctx, cfg, pool)
	if err != nil {
		log.Fatalf("wire app: %v", err)
	}

	r, err := newRouter(cfg)
	if err != nil {
		log.Fatalf("router: %v", err)
	}
	registerRoutes(r, a)

	// Blocks until SIGTERM/SIGINT, then shuts down HTTP + River.
	serve(ctx, newServer(cfg.Addr(), r), a.riverClient)
}
