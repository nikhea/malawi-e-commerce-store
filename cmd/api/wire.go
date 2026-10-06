package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nikhea/malawi-e-commerce-store/config"
	// Blank import: runs docs.init(), which registers the spec with the
	// swag registry. Without it /swagger/doc.json 500s — and nothing at
	// compile time warns you, since side-effect imports are invisible.
	_ "github.com/nikhea/malawi-e-commerce-store/docs"
	auth "github.com/nikhea/malawi-e-commerce-store/internal/auth"
	authpublic "github.com/nikhea/malawi-e-commerce-store/internal/auth/public"
	cart "github.com/nikhea/malawi-e-commerce-store/internal/cart"
	cartpublic "github.com/nikhea/malawi-e-commerce-store/internal/cart/public"
	categories "github.com/nikhea/malawi-e-commerce-store/internal/categories"
	categoriespublic "github.com/nikhea/malawi-e-commerce-store/internal/categories/public"
	inventory "github.com/nikhea/malawi-e-commerce-store/internal/inventory"
	inventorypublic "github.com/nikhea/malawi-e-commerce-store/internal/inventory/public"
	media "github.com/nikhea/malawi-e-commerce-store/internal/media"
	mediapublic "github.com/nikhea/malawi-e-commerce-store/internal/media/public"
	mediaservice "github.com/nikhea/malawi-e-commerce-store/internal/media/service"
	products "github.com/nikhea/malawi-e-commerce-store/internal/products"
	productspublic "github.com/nikhea/malawi-e-commerce-store/internal/products/public"
	users "github.com/nikhea/malawi-e-commerce-store/internal/users"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	variants "github.com/nikhea/malawi-e-commerce-store/internal/variants"
	variantspublic "github.com/nikhea/malawi-e-commerce-store/internal/variants/public"
	pkgmedia "github.com/nikhea/malawi-e-commerce-store/pkg/media"
	"github.com/nikhea/malawi-e-commerce-store/pkg/middleware"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// app is the fully assembled application: every module service plus the
// River client (for graceful shutdown in main). Built once by wireApp;
// main only reads its fields to register routes and stop the client.
type app struct {
	usersSvc      userspublic.Service
	categoriesSvc categoriespublic.Service
	variantsSvc   variantspublic.Service
	productsSvc   productspublic.Service
	cartSvc       cartpublic.Service
	inventorySvc  inventorypublic.Service
	authSvc       authpublic.Service
	mediaSvc      *mediaservice.Service
	riverClient   *river.Client[pgx.Tx]
}

// wireApp assembles modules and infra in dependency order:
// leaves (users, categories, variants) → products → uploader → River
// client → media pipeline → auth.
// Modules build via their index.go Wire entry points; only app-level
// infra (uploader, River, admin set) is constructed here.
func wireApp(ctx context.Context, cfg config.Config, pool *pgxpool.Pool) (*app, error) {
	usersSvc := users.Wire(pool)
	categoriesSvc := categories.Wire(pool)
	variantsSvc := variants.Wire(pool)
	productsSvc := products.Wire(pool, categoriesSvc, variantsSvc)
	cartSvc := cart.Wire(pool, productsSvc, usersSvc)
	inventorySvc := inventory.Wire(pool)

	uploader, err := pkgmedia.NewCloudinaryUploader(pkgmedia.CloudinaryConfig{
		CloudName: cfg.CloudName,
		APIKey:    cfg.CloudAPIKey,
		APISecret: cfg.CloudAPISecret,
	})
	if err != nil {
		return nil, fmt.Errorf("media uploader: %w", err)
	}

	workers := river.NewWorkers()
	riverClient, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 10},
		},
		Workers: workers,
	})
	if err != nil {
		return nil, fmt.Errorf("river client: %w", err)
	}

	mediaSvc := media.Wire(riverClient, uploader)
	river.AddWorker(workers, mediaSvc)
	// Completion callbacks: the worker calls these after a successful
	// upload. New image owners add one RegisterComplete line here.
	mediaSvc.RegisterComplete(mediapublic.OwnerCategory,
		func(ctx context.Context, ownerID, url, publicID string) error {
			_, err := categoriesSvc.SetImage(ctx, ownerID, url, publicID)
			return err
		})
	mediaSvc.RegisterComplete(mediapublic.OwnerProduct,
		func(ctx context.Context, ownerID, url, publicID string) error {
			_, err := productsSvc.SetImage(ctx, ownerID, url, publicID)
			return err
		})
	if err := riverClient.Start(ctx); err != nil {
		return nil, fmt.Errorf("river start: %w", err)
	}

	authSvc := auth.Wire(
		usersSvc,
		cfg.JWTSecret,
		time.Duration(cfg.JWTTTLHours)*time.Hour,
		adminEmailSet(cfg.AdminEmails),
	)

	return &app{
		usersSvc:      usersSvc,
		categoriesSvc: categoriesSvc,
		variantsSvc:   variantsSvc,
		productsSvc:   productsSvc,
		cartSvc:       cartSvc,
		inventorySvc:  inventorySvc,
		authSvc:       authSvc,
		mediaSvc:      mediaSvc,
		riverClient:   riverClient,
	}, nil
}

// adminEmailSet normalizes ADMIN_EMAILS for the auth bootstrap lookup.
// The service looks up the already-lowercased email, so the set must be
// lowercased too — otherwise "Boss@Shop.com" in env never matches.
func adminEmailSet(emails []string) map[string]struct{} {
	set := make(map[string]struct{}, len(emails))
	for _, e := range emails {
		if e = strings.ToLower(strings.TrimSpace(e)); e != "" {
			set[e] = struct{}{}
		}
	}
	return set
}

// newRouter builds the Gin engine with global middleware. Release mode in
// production (no debug logs or debug routes); trusted proxies cleared
// (direct exposure — set LB IPs here when deploying behind one, otherwise
// client IPs can't be trusted for rate limiting later).
func newRouter(cfg config.Config) (*gin.Engine, error) {
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery(), middleware.RequestID(), middleware.Logger())
	if err := r.SetTrustedProxies(nil); err != nil {
		return nil, fmt.Errorf("trusted proxies: %w", err)
	}
	return r, nil
}

// registerRoutes maps health, module APIs, and the Swagger UI.
func registerRoutes(r *gin.Engine, a *app) {
	// Infra check: raw {"status":"ok"}, no response envelope.
	// Versioned module APIs (/api/v1/…) use pkg/response instead.
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Open: register/login must NOT sit behind JWT. Catalog reads are
	// open too — storefront browsing needs no login.
	open := r.Group("/api/v1")
	auth.RegisterRoutes(open, a.authSvc)

	// Protected: JWT runs first (sets role + user id), then module routes.
	// RequireRole inside the admin groups enforces for real.
	protected := r.Group("/api/v1")
	protected.Use(middleware.JWT(a.authSvc))
	users.RegisterRoutes(protected, a.usersSvc)
	categories.RegisterRoutes(open, protected, a.categoriesSvc, a.mediaSvc)
	variants.RegisterRoutes(open, protected, a.variantsSvc)
	products.RegisterRoutes(open, protected, a.productsSvc, a.mediaSvc)
	cart.RegisterRoutes(protected, a.cartSvc)
	inventory.RegisterRoutes(protected, a.inventorySvc)

	// Generated API docs (docs/ is committed; refresh with
	// `swag init -g cmd/api/main.go --parseInternal -o docs`).
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))
}

// newServer builds the HTTP server with production timeouts: header reads
// capped against Slowloris, request/response bounds against hung handlers.
func newServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}

// serve runs srv until ctx ends, then shuts down HTTP and River together:
// stop taking traffic, finish in-flight requests, let active uploads drain.
func serve(ctx context.Context, srv *http.Server, riverClient *river.Client[pgx.Tx]) {
	go func() {
		log.Printf("listening on %s", srv.Addr)
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
	// Stop fetching new jobs, let active uploads finish.
	if err := riverClient.Stop(shutdownCtx); err != nil {
		log.Fatalf("river stop: %v", err)
	}
	log.Println("stopped")
}
