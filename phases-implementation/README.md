# Bulk Endpoints — Implementation Plan

This folder breaks the bulk-endpoints spec into five sequential phases. Each
phase gets its own branch and commit. Branches stack: phase 1 starts from the
current working branch, phase 2 from phase 1, and so on.

## Phase order

| Phase | Branch | Scope |
|---|---|---|
| 1 | `feat/bulk-phase-1-foundation` | Shared bulk kernel (caps, results, duplicates, logging), Redis idempotency middleware, `apperr` codes, config, inventory bulk set-stock + `variantIds` read |
| 2 | `feat/bulk-phase-2-products` | Product bulk create/update/delete, `GET /products?ids=`, product delete blocker policy |
| 3 | `feat/bulk-phase-3-variants-cart` | Variant bulk create/update, cart bulk add |
| 4 | `feat/bulk-phase-4-reviews-wishlist` | `GET /reviews/me`, wishlist bulk add/remove |
| 5 | `feat/bulk-phase-5-swagger-tests` | Swagger annotations + regenerated docs, route-order/403/idempotency tests, AGENTS build-progress entry, PR notes |

## Shared decisions (apply to every phase)

### Request/response shape

- Every bulk response is `{"data": {"results": [...], "summary": {...}}}`.
  `results` keep request order and carry `index`, `id`, `status`, `data`,
  `error`, plus the optional bulk-only flags `alreadyExists`,
  `alreadyRemoved`, `wouldDelete`, `blockers`.
- `status` is `ok`, `error`, or `rolled_back`. `summary` counts total,
  succeeded (`ok`), and failed (everything else).
- Whole-request failures (`400`/`409`/`422`) use the existing
  `pkg/response.Error` envelope and `apperr.Code`.
- Result metadata fields use the spec's camelCase names (`alreadyExists`,
  `wouldDelete`, `dryRun`, `atomic`). Fields reused from an existing DTO
  keep snake_case (`product_id`, `variant_id`, `qty`, `rating`,
  `price_cents`).

### Caps and validation

- `MAX_BULK_ITEMS` (default **50**) is loaded once in `config` and applied in
  `pkg/bulk`.
- Lower caps: product bulk-delete and cart bulk add **25**.
- Empty `items`/`updates`/`ids`/`productIds` → `400 VALIDATION_ERROR`.
- Over the cap → `400 BULK_LIMIT_EXCEEDED`.
- Duplicate ids or line keys in one request → `400 DUPLICATE_ITEM`.
- Query-string id lists accept repeated params and comma-separated values.

### Atomic writes

- `atomic: false` (default): each item runs independently; per-item failures
  are captured.
- `atomic: true`: validate everything first, then write in one repository
  transaction. On failure, every result is `rolled_back` and the first
  failing item carries the real error.

### Idempotency

- `Idempotency-Key` is accepted on all bulk writes.
- Redis key: `idem:v1:<userID>:<key>`, TTL 24h.
- Request hash covers method + path + raw body; same key + same hash replays
  the stored status/body; same key + different hash → `409 IDEMPOTENCY_CONFLICT`.
- Redis unavailable: log and continue (fail-open), matching the rate limiter.

### Dry run

- `dryRun: true` runs full validation and returns the projected result with
  no writes. Supported on product create/update/delete, variant
  create/update, and wishlist delete. Product delete also returns
  `wouldDelete` and `blockers`.

### Product delete policy

Single delete is a hard delete today; `order_items` has no product FK, so
order history survives. Bulk delete will **not** change single delete.
Bulk delete **blocks** products referenced by `order_items` with a per-item
`CONFLICT` and reports the count in `blockers` during `dryRun`. Cascading
rows (variants, images, stock, cart lines, wishlist links) are removed by
the existing FKs, exactly as single delete does.

### Logging/audit

One `log.Printf` line per bulk request:
`bulk user=<id> endpoint=<method path> total=<n> succeeded=<n> failed=<n> duration=<d> request_id=<id>`.
No per-item admin audit trail exists in the codebase today; the bulk log is
the audit record. This is called out in the phase-5 PR notes.

### Auth and route order

- Admin bulk routes sit on the existing `RequireRole(admin)` groups.
- Customer bulk routes are JWT-only and derive the user id from the token.
- Static paths (`/bulk`, `/bulk-delete`, `/me`) are registered before
  parameterized `/:id` siblings.

### Tests

Per endpoint: happy path, partial failure, `atomic: true` rollback, cap
exceeded, duplicates, idempotent replay, non-admin 403, and `dryRun` where
applicable. Service tests use in-memory fakes; middleware tests use
`miniredis`; handler tests use `httptest`.
