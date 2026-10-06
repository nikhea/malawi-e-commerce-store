// Package jobs holds recurring, cross-module River job handlers that must
// not live in any single module. Any client (API, worker) may schedule
// and work them; the work itself is idempotent.
package jobs

import (
	"context"

	"github.com/riverqueue/river"
)

// stockReleaser is the sliver of inventory the sweep needs.
type stockReleaser interface {
	ReleaseExpired(ctx context.Context) ([]string, error)
}

// orderCanceller is the sliver of orders the sweep needs.
type orderCanceller interface {
	CancelByRef(ctx context.Context, orderRef string) error
}

// ExpireReservationsArgs is the periodic sweep job. Empty args: the work
// is "sweep everything expired", scheduled by cron in cmd/worker.
type ExpireReservationsArgs struct{}

func (ExpireReservationsArgs) Kind() string { return "expire_reservations" }

// ExpireReservationsWorker frees expired stock holds and cancels their
// orders. Both calls are idempotent, so overlapping runs (multi-worker)
// converge instead of corrupting.
type ExpireReservationsWorker struct {
	river.WorkerDefaults[ExpireReservationsArgs]
	inventory stockReleaser
	orders    orderCanceller
}

func NewExpireReservationsWorker(inventory stockReleaser, orders orderCanceller) *ExpireReservationsWorker {
	return &ExpireReservationsWorker{inventory: inventory, orders: orders}
}

func (w *ExpireReservationsWorker) Work(ctx context.Context, job *river.Job[ExpireReservationsArgs]) error {
	refs, err := w.inventory.ReleaseExpired(ctx)
	if err != nil {
		return err
	}
	for _, ref := range refs {
		if err := w.orders.CancelByRef(ctx, ref); err != nil {
			return err
		}
	}
	return nil
}
