// Package jobs holds recurring, cross-module River job handlers that must
// not live in any single module. Any client (API, worker) may schedule
// and work them; the work itself is idempotent.
package jobs

import (
	"context"
	"errors"
	"fmt"

	"github.com/riverqueue/river"
)

// expiredLister is the sliver of inventory the sweep needs.
type expiredLister interface {
	ExpiredReservations(ctx context.Context) ([]string, error)
}

// expiredSettler is the sliver of orders the sweep needs. Settlement is
// status-aware, so a paid order confirms its hold instead of releasing it.
type expiredSettler interface {
	SettleExpired(ctx context.Context, orderRef string) error
}

// ExpireReservationsArgs is the periodic sweep job. Empty args: the work
// is "sweep everything expired", scheduled by cron in cmd/worker.
type ExpireReservationsArgs struct{}

func (ExpireReservationsArgs) Kind() string { return "expire_reservations" }

// ExpireReservationsWorker frees expired stock holds by asking orders to
// settle each ref. Listing and settling are separate calls, so a hold stays
// visible to the next run until it is actually settled — a failure halfway
// through cannot strand the rest.
type ExpireReservationsWorker struct {
	river.WorkerDefaults[ExpireReservationsArgs]
	inventory expiredLister
	orders    expiredSettler
}

func NewExpireReservationsWorker(inventory expiredLister, orders expiredSettler) *ExpireReservationsWorker {
	return &ExpireReservationsWorker{inventory: inventory, orders: orders}
}

func (w *ExpireReservationsWorker) Work(ctx context.Context, job *river.Job[ExpireReservationsArgs]) error {
	refs, err := w.inventory.ExpiredReservations(ctx)
	if err != nil {
		return err
	}
	// Keep going after a bad ref: returning early used to leave every
	// later order pending with its stock already released.
	var errs []error
	for _, ref := range refs {
		if err := w.orders.SettleExpired(ctx, ref); err != nil {
			errs = append(errs, fmt.Errorf("settle expired order %s: %w", ref, err))
		}
	}
	return errors.Join(errs...)
}
