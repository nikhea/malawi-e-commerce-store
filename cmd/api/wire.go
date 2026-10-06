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
	"github.com/nikhea/malawi-e-commerce-store/db"
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
	jobs "github.com/nikhea/malawi-e-commerce-store/internal/jobs"
	media "github.com/nikhea/malawi-e-commerce-store/internal/media"
	mediapublic "github.com/nikhea/malawi-e-commerce-store/internal/media/public"
	mediaservice "github.com/nikhea/malawi-e-commerce-store/internal/media/service"
	orders "github.com/nikhea/malawi-e-commerce-store/internal/orders"
	orderspublic "github.com/nikhea/malawi-e-commerce-store/internal/orders/public"
	payments "github.com/nikhea/malawi-e-commerce-store/internal/payments"
	paymentsgateway "github.com/nikhea/malawi-e-commerce-store/internal/payments/gateway"
	paymentspublic "github.com/nikhea/malawi-e-commerce-store/internal/payments/public"
	products "github.com/nikhea/malawi-e-commerce-store/internal/products"
	productspublic "github.com/nikhea/malawi-e-commerce-store/internal/products/public"
	reviews "github.com/nikhea/malawi-e-commerce-store/internal/reviews"
	reviewspublic "github.com/nikhea/malawi-e-commerce-store/internal/reviews/public"
	users "github.com/nikhea/malawi-e-commerce-store/internal/users"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	variants "github.com/nikhea/malawi-e-commerce-store/internal/variants"
	variantspublic "github.com/nikhea/malawi-e-commerce-store/internal/variants/public"
	wishlist "github.com/nikhea/malawi-e-commerce-store/internal/wishlist"
	wishlistpublic "github.com/nikhea/malawi-e-commerce-store/internal/wishlist/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/events"
	"github.com/nikhea/malawi-e-commerce-store/pkg/mail"
	pkgmedia "github.com/nikhea/malawi-e-commerce-store/pkg/media"
	"github.com/nikhea/malawi-e-commerce-store/pkg/middleware"
	"github.com/redis/go-redis/v9"
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
	ordersSvc     orderspublic.Service
	paymentsSvc   paymentspublic.Service
	wishlistSvc   wishlistpublic.Service
	reviewsSvc    reviewspublic.Service
	authSvc       authpublic.Service
	mediaSvc      *mediaservice.Service
	riverClient   *river.Client[pgx.Tx]
	redisClient   *redis.Client // nil when Redis is down: limiters detach
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

	// Redis backs rate limiting only. Fail-open by design: a dead Redis
	// disables throttling (loudly) instead of taking the shop down.
	redisClient, err := db.NewRedisClient(ctx, cfg.RedisURL, cfg.RedisHost, cfg.RedisPort, cfg.RedisDB, cfg.RedisPassword)
	if err != nil {
		log.Printf("WARNING: redis unavailable, rate limiting disabled: %v", err)
		redisClient = nil
	}

	// In-process event bus: orders publishes, payments and notification
	// senders will subscribe. No persistence — restart drops nothing
	// because events only trigger repeatable side effects.
	bus := events.New()
	ordersSvc := orders.Wire(pool, cartSvc, inventorySvc, bus)
	wishlistSvc := wishlist.Wire(pool, productsSvc, usersSvc)
	reviewsSvc := reviews.Wire(pool, productsSvc, usersSvc)
	paymentsSvc := payments.Wire(pool, ordersSvc,
		paymentsgateway.NewStripeGateway(cfg.StripeSecretKey), bus,
		cfg.StripeWebhookSecret, cfg.FXMWKPerUSD)

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
		// The expiry sweep is registered on EVERY client: River elects
		// one leader to schedule, and the job is idempotent, so
		// overlapping boots converge instead of double-processing.
		PeriodicJobs: []*river.PeriodicJob{
			river.NewPeriodicJob(
				river.PeriodicInterval(time.Minute),
				func() (river.JobArgs, *river.InsertOpts) {
					return jobs.ExpireReservationsArgs{}, nil
				},
				&river.PeriodicJobOpts{RunOnStart: true},
			),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("river client: %w", err)
	}

	mediaSvc := media.Wire(riverClient, uploader)
	river.AddWorker(workers, mediaSvc)
	// The expiry worker runs here too (same idempotent job): the sweep
	// works with api alone, worker alone, or both — whoever is up.
	river.AddWorker(workers, jobs.NewExpireReservationsWorker(inventorySvc, ordersSvc))
	// Mail worker registered (not just the worker binary): River REQUIRES
	// the kind in the inserting client's bundle, so the API must know it
	// too. Either process may deliver; River still runs each job once.
	sender := mail.NewSMTPSender("smtp.gmail.com", 587, cfg.EmailAddress, cfg.EmailPassword, cfg.EmailAddress)
	river.AddWorker(workers, jobs.NewSendMailWorker(sender))
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

	authSvc := auth.Wire(pool, usersSvc, bus,
		cfg.JWTSecret,
		time.Duration(cfg.JWTTTLHours)*time.Hour,
		adminEmailSet(cfg.AdminEmails),
	)

	// Bridge: the in-process bus is per-process, but mail must cross to
	// the worker. These subscribers convert mail events into durable
	// send_mail River jobs (persisted in Postgres). Log-only events stay
	// local — they prove the fan-out without leaving the process.
	bridgeMailEvents(bus, riverClient, usersSvc, ordersSvc, cfg.AppURL)

	return &app{
		usersSvc:      usersSvc,
		categoriesSvc: categoriesSvc,
		variantsSvc:   variantsSvc,
		productsSvc:   productsSvc,
		cartSvc:       cartSvc,
		inventorySvc:  inventorySvc,
		ordersSvc:     ordersSvc,
		paymentsSvc:   paymentsSvc,
		wishlistSvc:   wishlistSvc,
		reviewsSvc:    reviewsSvc,
		authSvc:       authSvc,
		mediaSvc:      mediaSvc,
		riverClient:   riverClient,
		redisClient:   redisClient,
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

// bridgeMailEvents converts mail-worthy domain events into durable
// send_mail River jobs. Recipient resolution happens here (fresh DB
// reads, not snapshots). Enqueue failures log — the event already
// happened; a lost mail beats a failed request.
func bridgeMailEvents(bus *events.Bus, riverClient *river.Client[pgx.Tx], users userspublic.Service, orders orderspublic.Service, appURL string) {
	enqueue := func(to, subject, body string) {
		if to == "" {
			return
		}
		_, err := riverClient.Insert(context.Background(), jobs.SendMailArgs{
			To: to, Subject: subject, Body: body,
		}, nil)
		if err != nil {
			log.Printf("bridge: enqueue mail to %s failed: %v", to, err)
		}
	}
	emailOf := func(ctx context.Context, userID string) string {
		u, err := users.GetByID(ctx, userID)
		if err != nil {
			return ""
		}
		return u.Email
	}

	bus.Subscribe(authpublic.EmailVerificationRequested, func(_ context.Context, e events.Event) {
		m, ok := e.Payload.(authpublic.VerificationMail)
		if !ok {
			return
		}
		name := m.Name
		if name == "" {
			name = m.Email
		}
		enqueue(m.Email, "Verify your email — Malawi Store",
			"Hi "+name+",\n\nYour verification code is: "+m.Code+
				"\n\nIt expires in 15 minutes.\n\n— Malawi Store")
	})
	bus.Subscribe(authpublic.PasswordResetRequested, func(_ context.Context, e events.Event) {
		m, ok := e.Payload.(authpublic.ResetMail)
		if !ok {
			return
		}
		enqueue(m.Email, "Reset your password — Malawi Store",
			"Hi "+m.Name+",\n\nReset your password here (valid 1 hour):\n"+
				appURL+"/reset-password?token="+m.Token+
				"\n\nDidn't ask? Ignore this mail.\n\n— Malawi Store")
	})
	bus.Subscribe(orderspublic.OrderPaid, func(ctx context.Context, e events.Event) {
		o, ok := e.Payload.(orderspublic.Order)
		if !ok || o.UserID == "" {
			return
		}
		enqueue(emailOf(ctx, o.UserID), "Payment received — Malawi Store",
			"Hi,\n\nWe received your payment for order "+o.ID+
				". Your items are being prepared.\n\n— Malawi Store")
	})
	bus.Subscribe(paymentspublic.PaymentFailed, func(ctx context.Context, e events.Event) {
		pay, ok := e.Payload.(paymentspublic.Payment)
		if !ok {
			return
		}
		o, err := orders.GetByRef(ctx, pay.OrderID)
		if err != nil {
			return
		}
		enqueue(emailOf(ctx, o.UserID), "Payment failed — Malawi Store",
			"Hi,\n\nYour payment for order "+pay.OrderID+
				" failed. Your cart is intact — try again.\n\n— Malawi Store")
	})
	bus.Subscribe(orderspublic.OrderCreated, func(_ context.Context, e events.Event) {
		log.Printf("event: %s", e.Name)
	})
	bus.Subscribe(orderspublic.OrderCancelled, func(_ context.Context, e events.Event) {
		log.Printf("event: %s", e.Name)
	})
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
	// Brute-force shield: 10 auth attempts/min/IP. Attached here (not in
	// the module) because the policy belongs to the app, not the module.
	authGroup := open.Group("/auth")
	if a.redisClient != nil {
		authGroup.Use(middleware.RateLimit(a.redisClient, middleware.RateLimitConfig{
			Name: "auth", Limit: 10, Window: time.Minute,
		}))
	}
	auth.RegisterRoutes(authGroup, a.authSvc)

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
	orders.RegisterRoutes(protected, a.ordersSvc)
	// Burst shield for Stripe retries: 120 webhook hits/min/IP. The HMAC
	// stays the real authentication; this only absorbs floods.
	webhooks := open.Group("/webhooks")
	if a.redisClient != nil {
		webhooks.Use(middleware.RateLimit(a.redisClient, middleware.RateLimitConfig{
			Name: "webhooks", Limit: 120, Window: time.Minute,
		}))
	}
	payments.RegisterRoutes(webhooks, protected, a.paymentsSvc)
	wishlist.RegisterRoutes(protected, a.wishlistSvc)
	reviews.RegisterRoutes(open, protected, a.reviewsSvc)

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
