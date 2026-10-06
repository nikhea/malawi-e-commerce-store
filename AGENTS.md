# AGENTS.md — Malawi E-Commerce Store

Modular monolith in Go. Gin HTTP API + background worker, PostgreSQL,
Redis, Stripe (payments/webhooks), Cloudinary (media), JWT auth.
Module path: `github.com/nikhea/malawi-e-commerce-store`. Go 1.27.1.

## 1. Repository layout

```
.
├── cmd/
│   ├── api/            # HTTP entrypoint. main.go builds the Gin engine,
│   │   └── main.go     # wires config→db→repositories→services→handlers→routes, serves :8080
│   └── worker/         # Background entrypoint (jobs: email, webhooks, inventory sync).
│                       # NOTE: currently a stub (`package worker`, no main). Must become
│                       # `package main` with its own wiring before it can run.
├── internal/           # All business modules. NEVER import internal/* from outside the module
│   │                   # except via that module's public/ package (see §3).
│   ├── auth/           # Reference layout — the only fully-scaffolded module (all dirs still empty):
│   │   ├── model/      # Persistence entities (DB shape, struct tags)
│   │   ├── dto/        # API + cross-module data shapes (request/response structs)
│   │   ├── repository/ # DB access only. Takes ctx + model, returns model/error. No business logic.
│   │   ├── service/    # Business logic. Unexported struct + exported constructor.
│   │   │               # Depends on repository interfaces and OTHER modules' public.Service.
│   │   ├── handler/    # Gin handlers. Thin: bind/validate → call service → write pkg/response.
│   │   ├── routes.go   # RegisterRoutes(g *gin.RouterGroup, svc public.Service). No logic.
│   │   ├── public/     # THE module's external contract (see §3). api.go: Service interface
│   │   │               # + shared DTOs + emitted event names.
│   │   ├── utils/      # Module-private helpers (token hashing, password rules, etc.)
│   │   └── test/       # Module tests (service + handler). Use per-module testdata here.
│   ├── users/          # Identity/profile data. Everyone may depend on users/public; users
│   │                   # itself depends on NOTHING (leaf of the dependency graph).
│   ├── categories/     # Catalog taxonomy (trees, slugs). Read mostly by products.
│   ├── products/       # Catalog core (SPU + media refs). Read by cart, orders, wishlist.
│   ├── variants/       # SKU-level data (size/color/price deltas). Owned by products conceptually,
│   │                   # split out for SKU scale. products/public re-exports what others need.
│   ├── inventory/      # Stock levels + reservations. orders reserves via inventory/public;
│   │                   # payments confirms/cancels via events (see §4).
│   ├── wishlist/       # User→product links. Depends on users/public + products/public.
│   ├── cart/           # Cart lines. Depends on products/public (price/availability) for validation.
│   ├── orders/         # Checkout + order state machine. Depends on cart, inventory, users publics;
│   │                   # emits OrderCreated / OrderPaid / OrderCancelled events.
│   ├── payments/       # Stripe intents, checkout sessions, webhook handling. Subscribes to
│   │                   # OrderCreated; publishes PaymentSucceeded/PaymentFailed.
│   └── reviews/        # Product reviews/ratings. Depends on users/public + products/public.
│                       # Writes never block the catalog read path.
├── pkg/                # Shared kernel. Domain-AGNOSTIC only. If it mentions products/orders/
│   ├── middleware/     # users, it belongs in a module's public/, NOT here.
│   └── response/       # Gin middleware (auth, logging, CORS) + standard JSON envelope.
│                       # Future: pkg/apperr (coded errors), pkg/events (in-process bus).
├── config/             # Env loading + typed Config struct (currently empty — build it here,
│                       # one loader used by BOTH cmd/api and cmd/worker).
├── db/
│   ├── migrations/     # SQL migrations, one direction per file (currently empty).
│   └── queries/        # Raw SQL / query files if not using an ORM everywhere (currently empty).
├── .air.toml           # Air live-reload: builds ./cmd/api → ./tmp/main, loads .env automatically.
├── .env                # Local secrets. GITIGNORED — never commit, never print in full.
├── .env.example        # Committed placeholder copy. Update it whenever a new key is added.
└── tmp/                # Air build output. GITIGNORED.
```

## 2. Module anatomy (every module follows `auth/`)

| Layer        | File(s)                              | Owns                                                | Must NOT contain                          |
|--------------|--------------------------------------|-----------------------------------------------------|-------------------------------------------|
| `model/`     | `user.go`, `order.go`, …             | DB entity structs, table/column mapping             | Business rules, HTTP code                 |
| `dto/`       | `request.go`, `response.go`          | Validation tags, JSON shapes, cross-module structs | SQL, Gin contexts                         |
| `repository/`| `postgres.go`, `redis.go`            | SQL/Redis calls, `ctx` threading, `sql.ErrNoRows` mapping | Business decisions                |
| `service/`   | `service.go`                         | Business logic, transactions, calls to other modules' `public.Service` | Gin, SQL strings |
| `handler/`   | `handler.go`                         | `c.ShouldBindJSON`, status codes, `pkg/response` envelope | Business logic, DB calls       |
| `routes.go`  | package `<module>`                   | `RegisterRoutes(g *gin.RouterGroup, svc public.Service)` — route table only | Logic |
| `public/`    | `api.go`                             | `Service` interface + method DTOs + event names the module emits | Implementations |
| `utils/`     | helpers                              | Private pure functions for this module              | Cross-module imports                      |
| `test/`      | `service_test.go`                    | Table-driven tests with fakes for repos and sibling `public.Service` | Real DB |

New modules start as `internal/<name>/.gitkeep`, then grow the layers above
top-down: `public/api.go` (contract first) → `service/` → `repository/` →
`handler/` + `routes.go`.

## 3. Module boundary — the load-bearing rule

- Code in `cmd/*` or `internal/<A>` may import **only**
  `github.com/nikhea/malawi-e-commerce-store/internal/<B>/public`.
  Importing `<B>/service`, `<B>/repository`, `<B>/handler`, `<B>/model`
  directly is a layering violation — reject it in review.
- `public/api.go` exposes three things and nothing else:
  1. `type Service interface { ... }` — every cross-module call, `ctx` first arg;
  2. the request/response DTOs those methods need;
  3. the domain events the module emits (e.g. `OrderCreated`).
- Each service asserts compliance at compile time:
  `var _ public.Service = (*service)(nil)`.
- Dependency direction (no cycles): `users` ← everyone; `products` ←
  `cart, wishlist, reviews, orders`; `inventory` ← `orders`;
  `orders` → `payments` (via events, §4). If A needs B and B needs A,
  the shared concept moves down into the lower module's `public/`.

## 4. Communication patterns

- **Synchronous (needs an answer now):** constructor-inject the sibling's
  `public.Service`. Example: `cart.NewService(cartRepo, productsSvc
  products.Public.Service)` then `productsSvc.GetPrice(ctx, skuID)` during
  cart validation; `orders` calls `inventorySvc.Reserve(ctx, items)` inside
  the checkout transaction.
- **Asynchronous (side effects):** publish to the in-process event bus
  (`pkg/events`, to be built). `orders` publishes `OrderCreated`;
  `payments` (charge), `inventory` (confirm reservation), and notification
  senders subscribe. Subscribers return no values and never block checkout.
- **HTTP surface:** `/health` → `200 {"status":"ok"}` (no auth, used by
  Air/docker checks). Versioned APIs live at `/api/v1/<module>` and are
  registered per-module via that module's `RegisterRoutes`.

## 5. Request lifecycle (HTTP)

```
Gin engine (cmd/api) → pkg/middleware (auth JWT, request-id, logging)
  → internal/<module>/routes.go → handler (bind dto, 4xx on bad input)
  → service (rules, transaction, cross-module public.Service calls, publish events)
  → repository (SQL/Redis) → pkg/response envelope → JSON
```

Worker lifecycle (target): `cmd/worker` boots same `config/` + `db/`,
subscribes to `pkg/events` and Stripe webhook queue, runs jobs with
structured logging. No HTTP server.

## 6. Environment

Copy `.env.example` → `.env` for local dev. Keys:

| Key | Purpose |
|-----|---------|
| `APP_PORT`, `APP_URL` | Listen port / public base URL (`:8080`) |
| `DATABASE_URL` | Postgres DSN (`postgresql://…?sslmode=disable` locally) |
| `REDIS_URL`, `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD`, `REDIS_DB` | Cache/sessions/rate-limit, local `redis://localhost:6379/0` |
| `JWT_SECRET`, `JWT_TTL_HOURS` | Auth signing key + TTL |
| `EMAIL_SERVICE`, `EMAIL_ADDRESS`, `EMAIL_PASSWORD` | SMTP/app-password for transactional mail |
| `CLOUDINARY_URL`, `CLOUD_NAME`, `CLOUD_API_KEY`, `CLOUD_API_SECRET`, `CLOUDINARY_UPLOAD_PRESET` | Media uploads |
| `STRIPE_SECRET_KEY`, `NEXT_PUBLIC_STRIPE_PUBLISHABLE_KEY`, `STRIPE_WEBHOOKS_SIGNING_SECRET`, `STRIPE_PRICE_PRO`, `STRIPE_PRICE_SCALE` | Payments, plans, webhook verification |
| `QR_SIGNING_SECRET` | Signed QR payloads |

Local Stripe webhooks:
`stripe listen --forward-to localhost:8080/api/v1/webhooks/stripe --events checkout.session.completed,customer.subscription.created,customer.subscription.updated,customer.subscription.deleted,invoice.payment_succeeded,invoice.payment_failed`

## 7. Commands

```bash
air                              # dev with live-reload (loads .env via .air.toml env_files)
go build ./...                   # full build — must be green before commit
go vet ./...                     # static checks — must be green before commit
go test ./...                    # all module tests (per-module: go test ./internal/<name>/...)
go test ./internal/cart/...      # focused module test
go mod tidy                      # after adding/removing imports
redis-cli -h localhost -p 6379 ping   # expect PONG (local redis:7 container)
```

## 8. Conventions for agents

- `ctx context.Context` is always the first parameter of service/repository
  methods; never store it in structs.
- Handlers stay thin (bind → service → respond). Status mapping: service
  returns typed errors (`pkg/apperr`), handler maps to 4xx/5xx.
- Transactions and cross-module consistency live in `service/`, never in
  `handler/` or `repository/`.
- Every `.go` file must declare its package — zero-byte `.go` files break
  `go build ./...` (known offenders, untracked: `internal/auth/reader.go`,
  `internal/auth/routes.go` at module root; real routes live in
  `internal/<module>/routes.go`).
- `fmt.Println` after a blocking `r.Run()` is unreachable — log startup
  BEFORE serving or use structured logging.
- Prefer constructor injection (`NewService(repo, siblingSvc)`) over globals
  so tests can pass fakes.
- Keep `pkg/` domain-free; keep `config/` the single env loader shared by
  `cmd/api` and `cmd/worker`.
- Verify with real execution: run the build/vet/test commands, hit
  `GET /health`, and report evidence. Do not claim green without running it.

## 9. Git workflow

- `git status --short` → `git diff` → stage only intended files
  (`git add <paths>`, never `git add -A` blindly). `.env`, `tmp/`,
  `.opencode/` are gitignored and must never be staged.
- Messages: `feat:`, `fix:`, `chore:` + concise subject
  (e.g. `chore: scaffold internal modules and add local redis env`).
- Never commit secrets. When adding an env key, update `.env.example`
  with a placeholder in the same commit.
- Commit only with all of `go build ./...`, `go vet ./...` green
  (and `go test ./...` once tests exist).
