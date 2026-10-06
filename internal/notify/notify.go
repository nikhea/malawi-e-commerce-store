// Package notify is the app's notification outbox: domain code calls its
// narrow methods, it enqueues durable send_mail River jobs. It replaces
// the old in-process event bus (pkg/events, deleted): same decoupling
// for callers, but delivery survives process boundaries and restarts.
package notify

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/nikhea/malawi-e-commerce-store/internal/jobs"
	orderspublic "github.com/nikhea/malawi-e-commerce-store/internal/orders/public"
	paymentspublic "github.com/nikhea/malawi-e-commerce-store/internal/payments/public"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	"github.com/riverqueue/river"
)

// Service sends notifications through River. One struct implements the
// notifier ports of auth, orders, and payments (asserted at wiring by
// assigning to each narrow interface).
type Service struct {
	client *river.Client[pgx.Tx]
	users  userspublic.Service
	orders orderspublic.Service
	appURL string
}

func New(client *river.Client[pgx.Tx], users userspublic.Service, orders orderspublic.Service, appURL string) *Service {
	return &Service{client: client, users: users, orders: orders, appURL: appURL}
}

// AttachOrders sets the orders dependency after construction. Needed
// because orders itself takes this notifier (wiring cycle); call once at
// wiring before serving. Until attached, PaymentFailed skips mail loudly.
func (s *Service) AttachOrders(orders orderspublic.Service) {
	s.orders = orders
}

func (s *Service) enqueue(ctx context.Context, to, subject, body string) {
	if to == "" {
		return
	}
	// The event already happened; a lost mail beats a failed request.
	// River retries the job itself once enqueued — this only guards the
	// single INSERT.
	if _, err := s.client.Insert(ctx, jobs.SendMailArgs{To: to, Subject: subject, Body: body}, nil); err != nil {
		log.Printf("notify: enqueue mail to %s failed: %v", to, err)
	}
}

func (s *Service) emailOf(ctx context.Context, userID string) string {
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return ""
	}
	return u.Email
}

// SendVerification mails an OTP code (auth register/resend path).
func (s *Service) SendVerification(ctx context.Context, userID, email, name, code string) {
	_ = userID
	if name == "" {
		name = email
	}
	s.enqueue(ctx, email, "Verify your email — Malawi Store",
		"Hi "+name+",\n\nYour verification code is: "+code+
			"\n\nIt expires in 15 minutes.\n\n— Malawi Store")
}

// SendReset mails a password-reset link (auth forgot path).
func (s *Service) SendReset(ctx context.Context, userID, email, name, token string) {
	_ = userID
	s.enqueue(ctx, email, "Reset your password — Malawi Store",
		"Hi "+name+",\n\nReset your password here (valid 1 hour):\n"+
			s.appURL+"/reset-password?token="+token+
			"\n\nDidn't ask? Ignore this mail.\n\n— Malawi Store")
}

// OrderCreated logs the fan-out (no mail yet — receipt goes on payment).
func (s *Service) OrderCreated(_ context.Context, o orderspublic.Order) {
	log.Printf("notify: order received %s", o.ID)
}

// OrderPaid mails the receipt to the order owner (fresh lookup).
func (s *Service) OrderPaid(ctx context.Context, o orderspublic.Order) {
	if o.UserID == "" {
		return
	}
	s.enqueue(ctx, s.emailOf(ctx, o.UserID), "Payment received — Malawi Store",
		"Hi,\n\nWe received your payment for order "+o.ID+
			". Your items are being prepared.\n\n— Malawi Store")
}

// OrderCancelled logs the fan-out.
func (s *Service) OrderCancelled(_ context.Context, o orderspublic.Order) {
	log.Printf("notify: order cancelled %s", o.ID)
}

// PaymentSucceeded is currently log-only (the OrderPaid receipt already
// covers the customer mail) — kept as a seam for risk/review mails.
func (s *Service) PaymentSucceeded(_ context.Context, p paymentspublic.Payment) {
	log.Printf("notify: payment succeeded %s", p.StripeIntentID)
}

// PaymentFailed mails the buyer (resolved via the order — the payment
// row only carries the order ref).
func (s *Service) PaymentFailed(ctx context.Context, p paymentspublic.Payment) {
	if s.orders == nil {
		log.Printf("notify: payment failed %s (orders not attached, mail skipped)", p.OrderID)
		return
	}
	o, err := s.orders.GetByRef(ctx, p.OrderID)
	if err != nil {
		return
	}
	s.enqueue(ctx, s.emailOf(ctx, o.UserID), "Payment failed — Malawi Store",
		"Hi,\n\nYour payment for order "+p.OrderID+
			" failed. Your cart is intact — try again.\n\n— Malawi Store")
}
