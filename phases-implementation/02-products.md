# Phase 2 — Products

Branch: `feat/bulk-phase-2-products`
Base: `feat/bulk-phase-1-foundation`

## Goal

Land the three admin product bulk writes plus the `ids` catalog filter.
This is the largest phase because product create/update/delete all need
validation helpers extracted from the existing single-item service methods.

## Scope

### 1. `POST /admin/products/bulk` (create, cap 50)

Body: `{ "atomic": false, "dryRun": false, "items": [dto.CreateProductRequest...] }`.

- Reuse `CreateProductRequest` unchanged.
- Extract `prepareCreate` from `service.Create`: trim name, slugify, price and
  currency rules, category existence.
- Duplicate generated/requested slugs inside the batch → whole-request
  `400 DUPLICATE_ITEM`.
- DB slug collision → per-item `CONFLICT`.
- `dryRun` returns the would-be `public.Product` (no ID) without writing.
- `atomic` writes every product in one repository transaction.
- Optional nested `variants: []` is **not** implemented in this phase. It is
  marked nice-to-have in the spec and would require a cross-module
  transaction. Report this in phase 5 PR notes.

### 2. `PATCH /admin/products/bulk` (update, cap 50)

Body: `{ "atomic": false, "dryRun": false, "updates": [{ "id": "...", "patch": {dto.UpdateProductRequest} }] }`.

- Extract `applyUpdate` from `service.Update` so bulk and single share patch
  semantics (partial, omitted = unchanged, slug immutable, `clear_category`).
- Empty patch per item → per-item `VALIDATION_ERROR`.
- Duplicate ids → whole-request `400 DUPLICATE_ITEM`.
- `dryRun` validates and returns the projected product without writing.
- `atomic` updates all rows in one transaction.

### 3. `POST /admin/products/bulk-delete` (cap 25)

Body: `{ "ids": ["..."], "dryRun": true, "atomic": false }`.

- POST rather than DELETE-with-body.
- Duplicate ids → `400 DUPLICATE_ITEM`.
- Policy: **block** with per-item `CONFLICT` when `order_items` references
  the product. Single delete stays unchanged.
- `dryRun` returns `wouldDelete` plus `blockers`:
  - `"referenced by N orders"`
  - `"has N variants"`
  - `"in N carts"`
  - `"in N wishlists"` (if present)
- Real delete hard-deletes; existing FKs cascade variants, gallery images,
  inventory, reservations, cart lines, and wishlist links.

### 4. `GET /products?ids=`

- Accept repeated `ids` and comma-separated ids, cap 50.
- Add `IDs []string` to `public.ListFilter` and `model.ListFilter`.
- Repository adds `p.id = ANY($n)` and combines with all existing filters
  and pagination. Same `public.ProductListResponse`.
- There is no `GET /admin/products` list today, so no admin counterpart.

### 5. Repository additions

- `CreateMany(ctx, []model.Product) ([]model.Product, int, error)`
- `UpdateMany(ctx, []model.Product) ([]model.Product, int, error)`
- `DeleteMany(ctx, ids) (int, error)`
- `Blockers(ctx, ids) (map[string]Blockers, error)`

`failIndex` is `-1` on success so the service can mark the correct item
`rolled_back`.

## Tests

- Create: happy path, slug conflict per item, duplicate slug in batch, cap,
  dry-run no writes, atomic rollback.
- Update: happy path, empty patch per item, unknown id per item, dry-run,
  atomic rollback, duplicate ids.
- Delete: happy path cascade, order-history blocker, dry-run blockers,
  cap 25, atomic rollback.
- List: `ids` repeated/comma, cap, combined filters.

## Acceptance

- Existing single create/update/delete unchanged.
- `go build ./...`, `go vet ./...`, `go test ./internal/products/...`.
- Commit: `feat: add admin product bulk create update delete endpoints`.
