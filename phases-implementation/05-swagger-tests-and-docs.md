# Phase 5 — Swagger, Integration Tests, Docs

Branch: `feat/bulk-phase-5-swagger-tests`
Base: `feat/bulk-phase-4-reviews-wishlist`

## Goal

Close the definition of done: every bulk route documented in Swagger, the
cross-cutting behaviors covered by tests, and the PR/AGENTS notes written.

## Scope

### 1. Swagger

- Add `swag` annotations to every new handler:
  - `POST /admin/inventory/stock/bulk`
  - `GET /admin/inventory/stock?variantIds=`
  - `POST /admin/products/bulk`
  - `PATCH /admin/products/bulk`
  - `POST /admin/products/bulk-delete`
  - `GET /products?ids=`
  - `POST /products/{id}/variants/bulk`
  - `PATCH /admin/variants/bulk`
  - `POST /cart/items/bulk`
  - `GET /reviews/me`
  - `POST /wishlist/bulk`
  - `POST /wishlist/bulk-delete`
- Register request models (`dto.Bulk*`) and concrete response models
  (`dto.Bulk*Response`). Tag admin routes `admin`, cart `cart`, reviews
  `reviews`, wishlist `wishlist`, catalog reads `products`.
- Regenerate `docs/` with:
  `swag init -g cmd/api/main.go --parseInternal -o docs`.

### 2. Route-order and auth tests

- Router tests proving `/bulk`, `/bulk-delete`, and `/me` are not captured
  as `/{id}`.
- `403 FORBIDDEN` for a non-admin caller on every admin bulk route.
- Customer routes scoped to the token (a body-supplied user id is ignored /
  not part of the DTO).

### 3. Idempotency integration tests

- Same key + same body → replayed response, side effect happens once.
- Same key + different body → `409 IDEMPOTENCY_CONFLICT`.
- Key scoped per user (same key, different user → independent).
- No `Idempotency-Key` → normal execution.

Use `miniredis` (already in `go.mod`) and wrap one real bulk handler in a
test router.

### 4. Cross-cutting endpoint test matrix

For each bulk endpoint, ensure the phase suites cover:

- happy path
- partial failure
- `atomic: true` rollback
- cap exceeded
- duplicates
- idempotent replay (middleware-level)
- non-admin `403`
- `dryRun` no side effects (product create/update/delete, variant
  create/update, wishlist delete)

### 5. Documentation

- Add a "Bulk endpoints" bullet to `AGENTS.md` §10 with the decisions:
  - product bulk delete blocks products with order history (single delete
    remains hard-delete),
  - idempotency is Redis `idem:v1:<user>:<key>` with a 24h TTL,
  - request field-name differences: inventory items use
    `product_id`/`variant_id` (stock is keyed by both, not variant alone),
    cart/wishlist/variant item payloads reuse existing snake_case DTOs,
    new bulk flags use `atomic`/`dryRun`/`alreadyExists`.
- Write the PR note in `phases-implementation/PR-NOTES.md`.
- Update `.env.example` (already done in phase 1) and any module READMEs.

### 6. Final verification

```bash
go build ./...
go vet ./...
go test ./...
swag init -g cmd/api/main.go --parseInternal -o docs
git status --short
```

Regenerated docs must be committed. Do not commit `.env`, `tmp/`, or
`.opencode/`.

## Acceptance

- All routes above documented and visible in `/swagger/doc.json`.
- No regression in single-item endpoints.
- All tests green.
- Commit: `feat: document bulk endpoints and add integration coverage`.
