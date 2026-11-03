# App Review Remediation — Implementation Plan

This folder tracks the fixes for the findings in `00-app-review.md`. The
batches are stacked: each branch starts from the previous one, so the work
can be reviewed and merged in order. One branch + one commit per batch.

| Batch | Branch | Scope | Status |
|---|---|---|---|
| 1 | `fix/01-stripe-webhook-secret-guard` | Fail closed when the Stripe webhook secret is missing; 5xx for server-side misconfig | done |
| 2 | `fix/02-payment-stock-lifecycle` | Align reservation/session TTLs, retry stock confirm on `MarkPaid`, cancel before release | in progress |
| 3 | `fix/03-expiry-sweep-resilience` | Sweep continues past failures, settles per order status, converges on retry | in progress |
| 4 | `fix/04-user-pii-and-reset-link` | Remove the customer-facing `GET /users/{id}`; reset links point at the frontend | in progress |
| 5 | `fix/05-auth-idempotency-hardening` | Atomic refresh/reset token use, cart currency guard, body limits, idempotency in-flight lock | in progress |
| 6 | `fix/06-worker-order-and-cleanup` | Register River workers before `Start`; low-severity cleanup | in progress |

## Ground rules

- No single-item endpoint behavior changes unless the finding is about that
  behavior (mixed-currency carts, `/users/{id}`).
- Every branch must be green on `go build ./...`, `go vet ./...` and
  `go test ./...` before its commit.
- Public contracts (`internal/<module>/public`) change only when the finding
  requires it; fakes in `internal/*/test` move with them.
- `AGENTS.md` §10 gets one bullet per landed batch.
- Never commit `.env`, `tmp/` or `.opencode/`.

## Batch notes

1. **Stripe webhook secret guard** — `config.Load` requires
   `STRIPE_WEBHOOKS_SIGNING_SECRET`; `HandleWebhook` rejects an empty secret
   with `INTERNAL` (5xx) so a misconfigured deployment retries instead of
   accepting forged events; signature failures stay 4xx. `.env.example`
   documents the key.
2. **Payment/stock lifecycle** — one reservation TTL constant in
   `orders/public` (35m), Checkout sessions expire at 30m and the hold is
   extended when the session is created. `MarkPaid` re-runs the idempotent
   stock confirm on repeat. Cancel claims the status transition before
   releasing stock.
3. **Expiry sweep** — inventory exposes expired refs without settling;
   `orders.SettleExpired` decides by order status (pending → cancel+release,
   paid → confirm, cancelled → release). The worker keeps going and joins
   errors so one poisoned ref cannot strand the rest.
4. **PII + reset link** — customer `GET /users/{id}` removed (admin keeps
   its own), reset mail built from `FRONTEND_URL`.
5. **Auth/idempotency** — refresh rotation is one conditional UPDATE, reset
   tokens are consumed atomically, carts reject a second currency, JSON
   bodies are capped, idempotency reads are capped and concurrent duplicates
   are rejected with 409.
6. **Worker/cleanup** — River workers registered before `Start`; JWT `exp`
   required, JWT secret length checked, `CreateReset` transactional,
   request-id sanitized, unused config removed, SMTP host/port configurable,
   Dockerfile fixes, gofmt, swagger typo, upload content-type sniffing.
