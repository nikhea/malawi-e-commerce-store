# App review — Malawi E-Commerce Store

Snapshot of the full-application review taken after bulk phase 5
(`ca43a86`). Findings are severity-ranked and include `file:line`
references. No code was changed by the review itself; the remediation is
tracked in `README.md` and the per-batch notes in this folder.

Baseline at review time: `go build ./...`, `go vet ./...` and
`go test ./...` all green; working tree clean; `gofmt -l` flagged only
`internal/payments/service/service.go`.

## Critical

### 1. Empty Stripe webhook secret = anyone can mark their own order paid

`config/config.go:124-130` only requires `DATABASE_URL` and `JWT_SECRET`.
`STRIPE_WEBHOOKS_SIGNING_SECRET` may be empty, and
`internal/payments/service/service.go:168` passes it straight into
`webhook.ConstructEventWithOptions` (`internal/payments/gateway/gateway.go:99`).
stripe-go computes HMAC-SHA256 with whatever string it is given — including
`""` — and has no empty-secret guard. The route is always mounted
(`cmd/api/wire.go`), so an attacker can compute
`t=<now>,v1=hmac_sha256("", "<payload>")`, send
`checkout.session.completed` with `metadata.order_id` set to their own order,
and `MarkPaid` runs. Free goods.

Fix: fail startup when the secret is empty; also hard-reject in
`HandleWebhook` (`if strings.TrimSpace(s.secret) == "" { return apperr.Internal(...) }`).

## High

### 2. 15-minute stock hold vs. 24-hour Checkout session

`internal/orders/index.go:20` sets `ReserveTTL: 15 * time.Minute`.
`CreateCheckoutSession` (`internal/payments/service/service.go:155-161`)
never sets `ExpiresAfter`, so Stripe uses its 24h default. The sweep cancels
the order and releases stock at minute 15 while the customer can still pay
at hour 3. The late webhook then hits `MarkPaid` → `CONFLICT("order is
cancelled")` and returns 400, so Stripe gives up: customer charged, order
cancelled, stock resold.

Fix: create the session with `ExpiresAfter` matching `ReserveTTL` (Stripe
minimum is 30 min, so raise the TTL to 30m and set the session expiry to the
same instant), or extend the reservation on session creation and honour
cancelled/expired sessions.

### 3. Expiry sweep strands every remaining order on the first failure

`internal/jobs/expire.go:46-49` returns on the first `CancelByRef` error.
But `ReleaseExpired` (`internal/inventory/repository/postgres.go:292-318`)
already settled *all* refs before returning, and its ref list is only the
ones that were active at SELECT time. River retries → second run finds no
expired holds → returns `nil` with zero refs. Every order after the poisoned
one stays `pending` forever with released stock. One paid/conflicting ref is
enough to trigger it.

Fix: accumulate errors and continue (`errors.Join`), and/or have
`CancelByRef` treat "already not pending" as success so it cannot poison the
loop.

### 4. `MarkPaid` marks paid before confirming stock, and never retries the confirm

`internal/orders/service/service.go:281-287`: `repo.MarkPaid` then
`ConfirmByOrder`; if confirm fails the error propagates, but the webhook
retry short-circuits at `case StatusPaid: return s.view(...)` (lines
271-272). The reservation is left `active`, the sweep later releases it, and
you ship a paid order whose inventory was never deducted.

Fix: in the `StatusPaid` branch, call `ConfirmByOrder` before returning (it
is documented idempotent), or move the order-status flip and stock settle
into one transaction.

### 5. Cancel releases stock before winning the status transition

`internal/orders/service/service.go:244-250`: `ReleaseByOrder` then
`repo.Cancel`. If a concurrent `MarkPaid` wins the guarded `pending → paid`
transition, the release has already freed stock for a paid order.

Fix: claim the transition first (`Cancel` returns the row only if it
flipped), then release; compensate by re-confirming if release fails, or
wrap both in a transaction.

## Medium

### 6. River worker registered after `Start` — data race

`cmd/api/wire.go:172` starts the client, then line 184 calls
`river.AddWorker(...)` on the same `*river.Workers`. The producer reads
`p.workers.workersMap` live when dispatching, so this is an unsynchronized
map write/read during normal traffic — Go can throw `fatal error: concurrent
map read and map write`. `cmd/worker/main.go:139-146` gets the order right.

Fix: register all workers before `Start` in the API too.

### 7. `GET /users/{id}` is an IDOR / PII leak

`internal/users/routes.go` mounts `users.GET("/:id", h.GetByID)` on the JWT
group. `internal/users/handler/handler.go:34-41` does no ownership or admin
check, and `public.User` (`internal/users/public/api.go:24-31`) includes
`email`, `role`, `email_verified`. Any logged-in customer can enumerate
accounts. `/users/me` already covers the legitimate use.

Fix: delete the route (keep the admin one) or require
`UserIDOf(c) == id || role == admin`.

### 8. Password-reset link points at the API origin

`internal/notify/notify.go:76` builds `s.appURL + "/reset-password?token=..."`.
`cmd/api/wire.go:180` passes `cfg.AppURL` — the API (`:8080`). The reset page
lives on `FRONTEND_URL` (`:3000`, already used correctly by payments). With
the documented config every reset mail 404s.

Fix: pass `cfg.FrontendURL` to `notify.New`.

### 9. Refresh rotation is racy

`internal/auth/service/service.go:238-271`: `GetRefresh` (no lock) → mint
replacement → `RotateRefresh`. Two concurrent refreshes with the same token
both succeed and create two live chains, defeating reuse detection.
`RotateRefresh` (`internal/auth/repository/postgres.go:135-145`) also lacks a
`revoked_at IS NULL` guard, so it happily re-rotates an already-revoked row.

Fix: make rotation a single conditional
`UPDATE ... WHERE id = $1 AND revoked_at IS NULL RETURNING ...` and treat 0
rows as reuse.

### 10. Reset-token double-use

`internal/auth/repository/postgres.go:83-104` SELECTs `password_resets`, then
UPDATEs `used_at` with no `AND used_at IS NULL` predicate and no transaction.
Concurrent requests both pass and both reset the password.

Fix: `UPDATE password_resets SET used_at = now() WHERE token_hash = $1 AND
used_at IS NULL AND expires_at > now() RETURNING user_id` and drive the reset
off that.

### 11. Mixed-currency cart totals

`internal/cart/service/service.go:43-50` sums `unit_price_cents` across lines
and keeps whichever line came last; `internal/cart/repository/postgres.go:95-105`
uses `MAX(currency)`; `UpsertItem` does not refresh `currency` on conflict
(`postgres.go:182-189`). A cart holding a USD line and a legacy MWK line
produces a meaningless subtotal, and orders copy it.

Fix: reject mixed-currency adds in `prepareAdd`, and make the conflict path
set `currency = EXCLUDED.currency`.

### 12. Idempotency: unbounded body read + no in-flight lock

`pkg/idempotency/idempotency.go:147-157` does `io.ReadAll` on the request
body with no limit, and there is no global `MaxBytesReader` anywhere — a
large POST is a memory-exhaustion vector. Separately, two concurrent
requests with the same key both miss the cache and both execute, so
`POST /cart/items/bulk` can double quantities.

Fix: wrap the body in `http.MaxBytesReader` (and add a global body limit in
middleware); add a short SETNX "in progress" key so concurrent duplicates get
409/retry.

### 13. Rate limiting collapses behind a proxy

`cmd/api/wire.go:242` sets `SetTrustedProxies(nil)`, so `c.ClientIP()` is the
load balancer's address and every user shares the 10/min auth limit. Fine for
direct exposure, a self-DoS behind a typical deployment.

Fix: make trusted proxies configurable (`TRUSTED_PROXIES`) and set it there.

### 14. Webhook config errors silently drop money events

`HandleWebhook` returns a 4xx for *any* verify failure
(`internal/payments/service/service.go:169-171`), so a rotated/wrong secret
after an incident makes Stripe permanently discard real
`checkout.session.completed` events. Forgery should be 400; "we are
misconfigured" should be 5xx so Stripe retries.

### 15. SDK does not implement the bulk surface it types

`sdk/src/types.ts` carries every bulk request/response type plus the
`relevance` sort key, but `sdk/src/resources/*.ts` expose no bulk methods and
`RequestOptions` has no per-request `Idempotency-Key`. `AGENTS.md` §10 claims
a full `/api/v1` mirror. Either add the methods (and an `idempotencyKey`
option) or soften the claim.

## Low

- `internal/auth/utils/jwt.go:27-37`: `exp` is read but never required; a
  token minted without `exp` is accepted (`MapClaims` only validates it when
  present) and the middleware does not check `claims.Expiry`. Require `exp`.
- `internal/auth/service/service.go` `Login` never checks `EmailVerified`;
  OTP `attempts` is incremented (`internal/auth/repository/postgres.go:58-61`)
  but no lockout is enforced.
- `config/config.go:118-120` uses `%w` with `nil` when `FX_MWK_PER_USD` is
  non-positive — prints `%!w(<nil>)`.
- `config/config.go:126-128` accepts any non-empty `JWT_SECRET`; require
  ≥32 bytes.
- `internal/auth/repository/postgres.go` `CreateReset` is delete-then-insert,
  non-transactional.
- `pkg/middleware/middleware.go` reflects the caller's `X-Request-ID` into
  logs unbounded/unvalidated (log injection, unbounded label).
- Unused config plumbing: `EmailService`, `StripePublishableKey`,
  `StripePricePro`, `StripePriceScale`, `QRSigningSecret` are loaded and never
  read; `.env.example` still advertises them, and `FRONTEND_URL` is missing
  from the AGENTS env table.
- `cmd/api/wire.go:154` hardcodes `smtp.gmail.com:587` instead of
  `EMAIL_SERVICE`.
- Dockerfile comment tells users to run `<image> /app/worker`, but the
  ENTRYPOINT means they need `--entrypoint /app/worker`; healthcheck hardcodes
  port 8080 even if `APP_PORT` differs.
- Uploads: no content-type sniffing on either the single or gallery path; a
  request can spill ~200 MiB of temp files (20 × 10 MiB) before the size check
  helps.
- N+1 reads: cart `toPublic` per-line work, product detail image fetch,
  reviews/wishlist list joins.
- No Postgres integration tests — every repository is faked, so the SQL
  (which is where most of the above races live) is untested.
- `gofmt -l` still flags `internal/payments/service/service.go` (misaligned
  struct fields).
- `internal/payments/handler/handler.go:91` has `@Produce js`.
- `internal/orders/model` reserves `pending` orders without a `currency`
  consistency check against line items.

## What is genuinely solid

- SQL injection is closed off properly: parameterized everywhere, sort keys
  whitelisted, and `utils.TSQuery` (`internal/products/utils/search.go`)
  rebuilds free text from alphanumeric runs so tsquery operators cannot be
  injected.
- JWT parsing pins HS256; auth is header-only, so the API is CSRF-immune by
  construction.
- CORS allowlist with credentials, security headers on every reply, and an
  SSRF guard (`pkg/ssrf`) with the DNS-rebinding limitation documented rather
  than hidden.
- Inventory `reserve`/`settle` run in transactions with row locks plus
  `CHECK (reserved_qty <= on_hand_qty)` — overselling is structurally blocked.
- Bulk transactions really are atomic (`CreateMany`/`UpdateMany`/`DeleteMany`),
  validation is shared with the single-item paths, and dry-run genuinely
  projects without writing.
- Layering holds: no `internal/<A>` imports another module's
  `service`/`repository`/`handler`; `public/api.go` contracts, compile-time
  assertions, and byte-for-byte route tests are in place.
- 403/401 coverage on every admin bulk route is locked by
  `cmd/api/router_test.go`.
