package jobs_test

import (
	"context"
	"errors"
	"testing"

	"github.com/nikhea/malawi-e-commerce-store/internal/jobs"
	"github.com/riverqueue/river"
)

type fakeInventory struct {
	refs []string
	err  error
}

func (f *fakeInventory) ExpiredReservations(context.Context) ([]string, error) {
	return f.refs, f.err
}

type fakeOrders struct {
	settled []string
	fail    map[string]error
}

func (f *fakeOrders) SettleExpired(_ context.Context, ref string) error {
	f.settled = append(f.settled, ref)
	if f.fail != nil {
		return f.fail[ref]
	}
	return nil
}

func TestExpireSweep(t *testing.T) {
	inv := &fakeInventory{refs: []string{"order-1", "order-2"}}
	ord := &fakeOrders{}
	w := jobs.NewExpireReservationsWorker(inv, ord)

	job := &river.Job[jobs.ExpireReservationsArgs]{Args: jobs.ExpireReservationsArgs{}}
	if err := w.Work(context.Background(), job); err != nil {
		t.Fatalf("work: %v", err)
	}
	if len(ord.settled) != 2 {
		t.Fatalf("expected 2 settlements, got %v", ord.settled)
	}
}

func TestExpireSweepContinuesAfterFailure(t *testing.T) {
	inv := &fakeInventory{refs: []string{"order-1", "order-2", "order-3"}}
	ord := &fakeOrders{fail: map[string]error{"order-1": errors.New("poison")}}
	w := jobs.NewExpireReservationsWorker(inv, ord)

	job := &river.Job[jobs.ExpireReservationsArgs]{Args: jobs.ExpireReservationsArgs{}}
	err := w.Work(context.Background(), job)
	if err == nil {
		t.Fatal("expected an error so River retries the sweep")
	}
	// The poisoned ref must not stop the rest: every later order still
	// settles, and the failed ref stays expired for the next run.
	if len(ord.settled) != 3 {
		t.Fatalf("sweep stopped early: %v", ord.settled)
	}
}

func TestExpireSweepListError(t *testing.T) {
	inv := &fakeInventory{err: errors.New("db down")}
	w := jobs.NewExpireReservationsWorker(inv, &fakeOrders{})
	job := &river.Job[jobs.ExpireReservationsArgs]{Args: jobs.ExpireReservationsArgs{}}
	if err := w.Work(context.Background(), job); err == nil {
		t.Fatal("expected the list error to propagate")
	}
}
