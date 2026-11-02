# Phase 1 — Foundation + Inventory

Branch: `feat/bulk-phase-1-foundation`
Base: current branch (`fix/variantless-cart-lines`)

## Goal

Build the shared bulk kernel every later phase depends on, then land the
smallest complete endpoint pair: admin inventory set-stock bulk and the
`variantIds` read extension.

## Scope

### 1. `pkg/apperr`

Add codes and constructors:

- `BULK_LIMIT_EXCEEDED` → 400
- `DUPLICATE_ITEM` → 400
- `IDEMPOTENCY_CONFLICT` → 409

`StatusOf` maps the first two to 400, the third to 409.

### 2. `pkg/bulk`

Domain-free helpers:

- `DefaultMaxItems = 50`, `SetMaxItems`, `MaxItems` (atomic int).
- `CheckCount(n, limit int) error`: empty → `VALIDATION_ERROR`, over limit →
  `BULK_LIMIT_EXCEEDED`.
- `ParseIDs(values []string, limit int) ([]string, error)`: accepts repeated
  and comma-separated values, trims blanks, caps the list, rejects empty.
- `HasDuplicate(keys []string) (string, bool)`.
- `Log(c *gin.Context, endpoint string, started time.Time, s BulkSummary)`.
- `Summary(results)` and `Rollback(results, failIndex, err)` helpers.

### 3. `pkg/bulk/dto`

Shared generic wire types:

- `BulkSummary { total, succeeded, failed }`
- `BulkItemError { code, message }`
- `BulkResult[T] { index, id, status, data, error, alreadyExists,
  alreadyRemoved, wouldDelete, blockers }`
- `BulkResponse[T] { results, summary }`
- `Empty` for delete results with no data.

Swagger: concrete named types in each module (`type BulkXResponse
bulkdto.BulkResponse[X]`) because swag 1.16.6 cannot parse generic
instantiations in annotations, but does resolve named generic aliases.

### 4. `pkg/idempotency`

Redis-backed middleware, TTL 24h:

- Skips requests without `Idempotency-Key`.
- Scopes keys per authenticated user id.
- Hashes method + path + raw body.
- Replays stored `{status, body}` when the hash matches.
- Returns `409 IDEMPOTENCY_CONFLICT` when the key is reused with a different
  hash.
- Caches only 2xx responses.
- Fail-open on Redis errors.
- `(*Store)(nil).Middleware()` is safe, so routes can accept a nil store in
  tests and when Redis is down.

### 5. `config`

Add `MaxBulkItems int` (`MAX_BULK_ITEMS`, default 50, positive). Update
`.env.example`. Call `bulk.SetMaxItems(cfg.MaxBulkItems)` from
`cmd/api/wire.go`.

### 6. Inventory bulk set-stock

- `POST /admin/inventory/stock/bulk`
  Body: `{ "atomic": false, "items": [{ "product_id": "...", "variant_id": "...", "qty": 120 }] }`
  (spec says `variantId`; the existing stock DTO requires `product_id` too,
  so the bulk item reuses `product_id`/`variant_id`).
- Cap 50, duplicates keyed by `product_id|variant_id`, `qty >= 0`.
- Unknown product/variant → per-item `NOT_FOUND`.
- Repository `BulkUpsertStock(ctx, items, atomic)`; non-atomic loops the
  existing upsert, atomic runs one transaction.
- Response `inventory/dto.BulkSetStockResponse`.

### 7. Inventory read extension

- `GET /admin/inventory/stock?variantIds=a&variantIds=b` (also comma lists).
- Cap 50. Response `{ "stock": [public.Stock...], "notFound": [...] }`.
- When `variantIds` is absent, the existing `product_id` single-stock
  behavior is unchanged.
- Known variant with no inventory row reads as zero stock (existing
  semantic); unknown variant lands in `notFound`.

## Tests

- `pkg/bulk`: empty, cap, duplicates, comma/repeated parsing.
- `pkg/idempotency`: replay, key reuse conflict, skip when header absent,
  fail-open with a dead client.
- Inventory service: happy path, partial failure, atomic rollback, cap,
  duplicates, `variantIds` not-found split.

## Acceptance

- `go build ./...`, `go vet ./...`, `go test ./internal/inventory/... ./pkg/bulk/... ./pkg/idempotency/...`.
- Existing single-stock GET/POST behavior unchanged.
- Commit: `feat: add bulk kernel and inventory bulk stock endpoints`.
