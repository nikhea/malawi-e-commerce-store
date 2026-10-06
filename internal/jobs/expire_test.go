package jobs_test

import (
	"context"
	"testing"

	"github.com/nikhea/malawi-e-commerce-store/internal/jobs"
	"github.com/riverqueue/river"
)

type fakeInventory struct {
	refs []string
}

func (f *fakeInventory) ReleaseExpired(context.Context) ([]string, error) {
	return f.refs, nil
}

type fakeOrders struct {
	cancelled []string
}

func (f *fakeOrders) CancelByRef(_ context.Context, ref string) error {
	f.cancelled = append(f.cancelled, ref)
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
	if len(ord.cancelled) != 2 {
		t.Fatalf("expected 2 cancellations, got %v", ord.cancelled)
	}
}
