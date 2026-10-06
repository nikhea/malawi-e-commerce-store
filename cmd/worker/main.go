package main

// Worker: background job processing for the modular monolith.
//
// Runs WITHOUT an HTTP server:
//   - works the River "default" queue (media uploads, send_mail),
//   - sweeps expired stock reservations every minute (releases holds,
//     cancels their orders).
//
// The API (cmd/api) enqueues; this process works. Either or both may run:
// River distributes each job once. Local dev: `air` in one terminal,
// `go run ./cmd/worker` in another.

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nikhea/malawi-e-commerce-store/config"
	"github.com/nikhea/malawi-e-commerce-store/db"
	cart "github.com/nikhea/malawi-e-commerce-store/internal/cart"
	categories "github.com/nikhea/malawi-e-commerce-store/internal/categories"
	inventory "github.com/nikhea/malawi-e-commerce-store/internal/inventory"
	"github.com/nikhea/malawi-e-commerce-store/internal/jobs"
	media "github.com/nikhea/malawi-e-commerce-store/internal/media"
	mediapublic "github.com/nikhea/malawi-e-commerce-store/internal/media/public"
	orders "github.com/nikhea/malawi-e-commerce-store/internal/orders"
	orderspublic "github.com/nikhea/malawi-e-commerce-store/internal/orders/public"
	products "github.com/nikhea/malawi-e-commerce-store/internal/products"
	users "github.com/nikhea/malawi-e-commerce-store/internal/users"
	variants "github.com/nikhea/malawi-e-commerce-store/internal/variants"
	"github.com/nikhea/malawi-e-commerce-store/pkg/mail"
	pkgmedia "github.com/nikhea/malawi-e-commerce-store/pkg/media"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
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

	if err := run(ctx, cfg, pool); err != nil {
		log.Fatalf("worker: %v", err)
	}
}

// discardNotifier drops order-lifecycle fan-out. The worker's orders
// service needs a notifier to construct, but this process originates no
// user requests — mail for API-side checkouts is enqueued by the API's
// own notifier, and expiry cancellations stay silent by design.
type discardNotifier struct{}

func (discardNotifier) OrderCreated(context.Context, orderspublic.Order)   {}
func (discardNotifier) OrderPaid(context.Context, orderspublic.Order)      {}
func (discardNotifier) OrderCancelled(context.Context, orderspublic.Order) {}

func run(ctx context.Context, cfg config.Config, pool *pgxpool.Pool) error {
	// Module services the worker needs. Same index.go Wire entry points
	// as the API — assembly stays identical on both sides.
	usersSvc := users.Wire(pool)
	categoriesSvc := categories.Wire(pool)
	variantsSvc := variants.Wire(pool)
	productsSvc := products.Wire(pool, categoriesSvc, variantsSvc)
	cartSvc := cart.Wire(pool, productsSvc, usersSvc)
	inventorySvc := inventory.Wire(pool)

	// Worker needs no notifier: it WORKS the mail queue (send_mail jobs)
	// rather than enqueueing. Expiry cancellations here stay silent —
	// receipt/OTP mail originates from the API side that owns the user
	// request. A discard notifier keeps orders constructible.
	ordersSvc := orders.Wire(pool, cartSvc, inventorySvc, discardNotifier{})

	// SMTP sender for the send_mail queue. Misconfigured mail does NOT
	// stop the worker (jobs still process); sends just log failures.
	sender := mail.NewSMTPSender("smtp.gmail.com", 587, cfg.EmailAddress, cfg.EmailPassword, cfg.EmailAddress)

	uploader, err := pkgmedia.NewCloudinaryUploader(pkgmedia.CloudinaryConfig{
		CloudName: cfg.CloudName,
		APIKey:    cfg.CloudAPIKey,
		APISecret: cfg.CloudAPISecret,
	})
	if err != nil {
		return err
	}

	workers := river.NewWorkers()
	riverClient, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 10},
		},
		Workers: workers,
		PeriodicJobs: []*river.PeriodicJob{
			// Abandoned-checkout sweep. Idempotent end to end, so a
			// multi-worker fleet converging on the same rows is safe.
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
		return err
	}

	mediaSvc := media.Wire(riverClient, uploader)
	river.AddWorker(workers, mediaSvc)
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
	river.AddWorker(workers, jobs.NewExpireReservationsWorker(inventorySvc, ordersSvc))
	// send_mail delivery lives here. The API only enqueues (its notifier
	// adapter); this registration is what actually sends.
	river.AddWorker(workers, jobs.NewSendMailWorker(sender))

	if err := riverClient.Start(ctx); err != nil {
		return err
	}
	log.Println("worker started: media uploads + reservation sweep every minute")

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := riverClient.Stop(shutdownCtx); err != nil {
		return err
	}
	log.Println("worker stopped")
	return nil
}
