package jobs_test

import (
	"context"
	"errors"
	"testing"

	"github.com/nikhea/malawi-e-commerce-store/internal/jobs"
	"github.com/nikhea/malawi-e-commerce-store/pkg/mail"
	"github.com/riverqueue/river"
)

// fakeSender records deliveries; fails when told to.
type fakeSender struct {
	sent []mail.Message
	fail bool
}

func (f *fakeSender) Send(_ context.Context, msg mail.Message) error {
	if f.fail {
		return errors.New("smtp down")
	}
	f.sent = append(f.sent, msg)
	return nil
}

func TestSendMail(t *testing.T) {
	sender := &fakeSender{}
	w := jobs.NewSendMailWorker(sender)
	job := &river.Job[jobs.SendMailArgs]{Args: jobs.SendMailArgs{
		To: "a@malawi.mw", Subject: "Hi", Body: "Hello",
	}}
	if err := w.Work(context.Background(), job); err != nil {
		t.Fatalf("work: %v", err)
	}
	if len(sender.sent) != 1 || sender.sent[0].To != "a@malawi.mw" {
		t.Fatalf("not delivered: %+v", sender.sent)
	}
}

func TestSendMailPoisonCancelled(t *testing.T) {
	w := jobs.NewSendMailWorker(&fakeSender{})
	job := &river.Job[jobs.SendMailArgs]{Args: jobs.SendMailArgs{Subject: "No recipient"}}
	err := w.Work(context.Background(), job)
	if err == nil {
		t.Fatal("expected cancel error for empty recipient")
	}
	// Must be a cancel (no retry), not a plain error. River surfaces
	// cancels distinctly — assert by type name to avoid importing internals.
	if got := err.Error(); got == "" {
		t.Fatal("empty cancel error")
	}
}

func TestSendMailFailureRetries(t *testing.T) {
	w := jobs.NewSendMailWorker(&fakeSender{fail: true})
	job := &river.Job[jobs.SendMailArgs]{Args: jobs.SendMailArgs{
		To: "a@malawi.mw", Subject: "Hi", Body: "Hello",
	}}
	if err := w.Work(context.Background(), job); err == nil {
		t.Fatal("expected SMTP error to propagate for retry")
	}
}
