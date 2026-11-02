# Bulk Endpoints — PR Notes

## What this adds

Twelve bulk/read endpoints across the existing modules, built on a shared
`pkg/bulk` kernel and a Redis idempotency middleware:

| Method | Path | Auth | Cap |
|---|---|---|---|
| `POST` | `/api/v1/admin/inventory/stock/bulk` | admin | `MAX_BULK_ITEMS` (50) |
| `GET` | `/api/v1/admin/inventory/stock?variantIds=` | admin | 50 |
| `POST` | `/api/v1/admin/products/bulk` | admin | 50 |
| `PATCH` | `/api/v1/admin/products/bulk` | admin | 50 |
| `POST` | `/api/v1/admin/products/bulk-delete` | admin | 25 |
| `GET` | `/api/v1/products?ids=` | public | 50 |
| `POST` | `/api/v1/products/{id}/variants/bulk` | admin | 50 |
| `PATCH` | `/api/v1/admin/variants/bulk` | admin | 50 |
| `POST` | `/api/v1/cart/items/bulk` | JWT | 25 |
| `GET` | `/api/v1/reviews/me` | JWT | paged (max 100) |
| `POST` | `/api/v1/wishlist/bulk` | JWT | 50 |
| `POST` | `/api/v1/wishlist/bulk-delete` | JWT | 50 |

Every bulk response is `{"data": {"results": [...], "summary": {...}}}`.
Results keep request order and carry `index`, `id`, `status`, `data`,
`error`, plus the bulk-only flags `alreadyExists`, `alreadyRemoved`,
`wouldDelete`, `blockers`. HTTP 200 means the batch was processed; 400/409
mean the whole request was invalid.

## Decisions worth reviewing

### Product delete policy

- Single `DELETE /admin/products/{id}` stays a hard delete, unchanged.
- Bulk delete **blocks** products referenced by `order_items` with a per-item
  `CONFLICT`, because order history has no product FK and would otherwise
  lose its catalog reference.
- `dryRun` returns `wouldDelete` plus `blockers` (`orders`, `variants`,
  `cart`, `wishlist`) without writing.
- Cascading rows (variants, images, stock, cart/wishlist links) still go
  through the existing FKs, exactly like single delete.

### Idempotency storage

- Redis key: `idem:v1:<userID>:<key>`, TTL 24h, per-user scope.
- Hash covers method + URI + raw body. Same key + same hash replays the
  stored 2xx status/body; same key + different hash returns
  `409 IDEMPOTENCY_CONFLICT` without running the handler.
- Only 2xx responses are cached; a 400/409 should be fixed, not replayed.
- Redis errors fail open (log and continue), matching the rate limiter.
- The middleware is nil-safe, so tests and Redis-less deployments keep
  working.

### DTO field-name differences

- New bulk flags use the spec's camelCase: `atomic`, `dryRun`,
  `alreadyExists`, `alreadyRemoved`, `wouldDelete`.
- Inventory items carry both `product_id` and `variant_id`, because stock is
  keyed by the pair, not by variant alone.
- Cart, variant, wishlist and product item payloads reuse the existing
  snake_case DTO fields (`product_id`, `variant_id`, `qty`, `price_cents`,
  `is_active`, …).
- `POST /wishlist/bulk` takes `productIds` (the one intentional camelCase
  request field, per the spec). `POST /wishlist/bulk-delete` takes `ids`,
  which are **product ids**, matching single `DELETE /wishlist/{id}`.
- Variant bulk create nests under its product (`POST /products/{id}/variants/bulk`);
  bulk update is admin-keyed (`PATCH /admin/variants/bulk`).

### Swagger

Swag 1.16.6 cannot resolve generic instantiations in annotations and does not
parse the nested `pkg/bulk/dto` package. Therefore:

- The shared wire shapes (`BulkSummary`, `BulkItemError`, `BulkResult[T]`,
  `BulkResponse[T]`, `Empty`) moved from `pkg/bulk/dto` to `pkg/bulk`, which
  swag resolves. Runtime code still uses the generic types.
- Each module declares concrete response models (`dto.BulkProductResult`,
  `dto.BulkSetStockResponse`, …) plus a small projection constructor, so
  `/swagger/doc.json` shows real fields instead of `{"type": "object"}`.
- `GET /admin/inventory/stock` documents both modes through the
  `variantIds` parameter; the annotated 200 is the batch shape.

### Out of scope (unchanged)

Orders/checkout/payments/auth/user roles, admin review deletion, image
uploads, inventory reserve/confirm/release, category writes, and all
single-item endpoint behavior. Product bulk-create does not accept nested
`variants` (that would need a cross-module transaction).

## Tests

- Service suites per module: happy path, partial failure, atomic rollback,
  cap, duplicates, dry run where applicable.
- `cmd/api/router_test.go`: route-table presence for all twelve endpoints,
  403 `FORBIDDEN` for a customer on every admin bulk route, 401 without
  identity.
- `internal/wishlist/test/handler_test.go`: real route table + `miniredis`
  for replay, key/body conflict, per-user key scope, no-key passthrough,
  token-scoped identity, and static-path dispatch (`/bulk`, `/bulk-delete`).
- `internal/reviews/test/handler_test.go`: `GET /reviews/me` uses token
  identity and forwards paging/sort.

## Verification

```bash
go build ./...
go vet ./...
go test ./...
swag init -g cmd/api/main.go --parseInternal -o docs
```
