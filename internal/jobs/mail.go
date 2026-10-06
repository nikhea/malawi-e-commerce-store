package jobs

import (
	"context"
	"fmt"
	"log"

	"github.com/nikhea/malawi-e-commerce-store/pkg/mail"
	"github.com/riverqueue/river"
)

// SendMailArgs is one outbound email as a River job. Persisted in
// Postgres, so it crosses process boundaries (API enqueues, worker
// sends) — unlike the in-process event bus, which is per-process.
// All fields exported: River serializes args as JSON.
type SendMailArgs struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// Kind identifies the job in River.
func (SendMailArgs) Kind() string { return "send_mail" }

// SendMailWorker delivers one email. SMTP failures return errors so
// River retries with backoff — transient outages converge.
type SendMailWorker struct {
	river.WorkerDefaults[SendMailArgs]
	sender mail.Sender
}

func NewSendMailWorker(sender mail.Sender) *SendMailWorker {
	return &SendMailWorker{sender: sender}
}

func (w *SendMailWorker) Work(ctx context.Context, job *river.Job[SendMailArgs]) error {
	args := job.Args
	if args.To == "" || args.Subject == "" || args.Body == "" {
		// Poison job (programmer error, never transient): cancel, don't
		// burn 25 retries on something that can't succeed.
		return river.JobCancel(fmt.Errorf("jobs: send_mail missing to/subject/body"))
	}
	if err := w.sender.Send(ctx, mail.Message{To: args.To, Subject: args.Subject, Body: args.Body}); err != nil {
		return err
	}
	log.Printf("jobs: mailed %q to %s", args.Subject, args.To)
	return nil
}
