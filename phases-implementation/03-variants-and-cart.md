# Phase 3 — Variants + Cart

Branch: `feat/bulk-phase-3-variants-cart`
Base: `feat/bulk-phase-2-products`

## Goal

Add SKU bulk writes for admins and the customer cart bulk add.

## Scope

### 1. `POST /products/{id}/variants/bulk` (cap 50)

Path prefix follows the existing single create (`POST /products/{id}/variants`),
as the spec instructs, not `/admin/products/...`.

Body: `{ "atomic": false, "dryRun": false, "items": [dto.CreateVariantRequest...] }`.

- Product must exist → whole-request `404` before any item runs.
  `variants` remains a leaf module: existence is checked by the variants
  repository (`ProductExists`), not by importing products.
- SKU is uppercased like single create.
- Duplicate SKUs inside the batch → whole-request `400 DUPLICATE_ITEM`.
- DB SKU collision → per-item `CONFLICT`.
- `dryRun` validates and returns projected variants (no IDs) without writes.
- `atomic` inserts all SKUs in one transaction.
- Route is registered on the existing admin-gated nested group.

### 2. `PATCH /admin/variants/bulk` (cap 50)

Body: `{ "atomic": false, "dryRun": false, "updates": [{ "id": "...", "patch": {dto.UpdateVariantRequest} }] }`.

- Reuse single patch semantics: name, price, `is_active`; `product_id` and
  `sku` stay immutable.
- Empty patch per item → per-item `VALIDATION_ERROR`.
- Duplicate ids → whole-request `400 DUPLICATE_ITEM`.
- Availability is read before writes, mirroring single update, so a stock
  lookup failure cannot leave a committed write with a 500.
- `dryRun` returns projected variants with current availability, no writes.
- `atomic` updates all rows in one transaction.

### 3. `POST /cart/items/bulk` (cap 25)

Body: `{ "atomic": false, "items": [dto.AddItemRequest...] }`.

- Identity from JWT only; body never carries a user id.
- Extract `prepareAdd` from `service.AddItem` so one line keeps the same
  catalog validation, price snapshot, variant rules, and inactive checks.
- Duplicate line keys (`product_id|variant_id`) → whole-request
  `400 DUPLICATE_ITEM`.
- Non-atomic: each valid line upserts independently; invalid lines are
  reported per item and do not stop the batch.
- Atomic: validate every line first, then one transaction upserts all lines
  and touches the cart. Any failure rolls back every line.
- Response adds the full updated cart:
  `{ "results": [...], "summary": {...}, "cart": public.Cart }`.
- Cap 25 (spec lower cap).

## Tests

- Variants: product missing 404, duplicate SKU batch, DB conflict per item,
  dry-run, atomic rollback, update empty patch, cap.
- Cart: happy path, per-item invalid product, atomic rollback leaves cart
  unchanged, duplicate lines, cap 25, merge with existing line.

## Acceptance

- Existing single variant create/update and single cart add unchanged.
- `go build ./...`, `go vet ./...`, `go test ./internal/variants/... ./internal/cart/...`.
- Commit: `feat: add variant and cart bulk endpoints`.
