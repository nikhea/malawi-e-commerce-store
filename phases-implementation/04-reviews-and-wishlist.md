# Phase 4 — Reviews + Wishlist

Branch: `feat/bulk-phase-4-reviews-wishlist`
Base: `feat/bulk-phase-3-variants-cart`

## Goal

Ship the two remaining customer surfaces: the caller's review history and
wishlist bulk add/remove.

## Scope

### 1. `GET /reviews/me`

- JWT-only; identity from the token.
- Paginated with the existing `pagination.Meta` conventions (default page
  size 12, max 100). Newest first by default; allow
  `newest|oldest|highest|lowest`.
- Each row includes review fields (`id`, `user_id`, `product_id`,
  `rating`, `title`, `body`, `created_at`, `updated_at`) plus product context:
  `product.id`, `product.name`, `product.slug`, `product.image_url`.
- Repository joins `reviews` to `products` and counts with the same owner
  predicate.
- Route registered before `/reviews/{id}`.

### 2. `POST /wishlist/bulk` (cap 50)

Body: `{ "productIds": ["..."], "atomic": false }`.

- JWT-only, caller-scoped.
- Product existence + active check through `products/public`; unknown or
  inactive → per-item `NOT_FOUND`.
- Already-wishlisted rows are `status: "ok"` with `alreadyExists: true`,
  never an error.
- Duplicate product ids in one request → whole-request `400 DUPLICATE_ITEM`.
- Repository `Add` now reports `(item, created, error)`; the single `Add`
  ignores the flag.
- `atomic` inserts all new links in one transaction; every result
  `rolled_back` on failure.

### 3. `POST /wishlist/bulk-delete` (cap 50)

Body: `{ "ids": ["..."], "dryRun": false, "atomic": false }`.

- Identifier matches the existing `DELETE /wishlist/{id}`: **product id**,
  not wishlist row id.
- Foreign/unknown product id → per-item `NOT_FOUND` (no existence leak).
- Known product not currently hearted → `status: "ok"` with
  `alreadyRemoved: true` (idempotent).
- `dryRun: true` reports `wouldDelete` per item and writes nothing.
- `atomic` removes all rows in one transaction.
- Route registered before `/:id`.

## Tests

- Reviews: only the caller's rows, product context attached, pagination,
  newest default.
- Wishlist add: happy path, partial unknown product, alreadyExists,
  duplicates, cap, atomic rollback.
- Wishlist delete: happy path, unknown id per item, alreadyRemoved,
  dry-run no writes, atomic rollback, cap.

## Acceptance

- Existing single wishlist add/remove and review routes unchanged.
- `go build ./...`, `go vet ./...`, `go test ./internal/reviews/... ./internal/wishlist/...`.
- Commit: `feat: add my-reviews and wishlist bulk endpoints`.
