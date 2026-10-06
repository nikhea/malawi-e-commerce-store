# users module

Single-storefront identity store. Leaf of the dependency graph: this module
depends on NOTHING, and `auth`, `orders`, `wishlist`, `cart`, `reviews`
depend on it via `public/` only. One store, no sellers — access control is
the `role` flag (`admin` | `customer`), not a separate tenants model.

Layout follows AGENTS.md §2 (module anatomy). The only cross-module
surface is `public/api.go`; everything else is module-private.

## Files

### `public/api.go` — the contract (front door)
Zero logic. Holds the `Role` type, the `User` view struct other modules
may see (no password hash field — the hash must never cross the module
boundary), `CreateUserInput`, and the `Service` interface
(Create/GetByID/GetByEmail/SetRole, `ctx` first arg). Every other module
and `cmd/*` imports ONLY this file. This is the promise the module makes
to the rest of the app.

### `model/user.go` — the database shape
Mirrors the `users` table row-for-row, including `PasswordHash`.
Module-private: only `repository` and `service` use it. The split from
`public.User` keeps persistence details (hashes, unexposed timestamps)
out of cross-module code. When the table gains a column, touch this
file first.

### `repository/postgres.go` — database access, nothing else
Four pgx queries: `Create` (INSERT…RETURNING), `GetByID`, `GetByEmail`,
`SetRole` (UPDATE…RETURNING). Two rules live here: emails are lowercased
on write/read so the `UNIQUE` constraint behaves case-insensitively, and
driver outcomes become coded errors — "no rows" → `apperr.NotFound`,
Postgres `23505` (unique violation) → `apperr.Conflict`. No business
decisions here: it doesn't know what a valid email looks like, only how
to store one.

### `service/service.go` — the brains
Validates then delegates. `Create` checks email format (stdlib
`net/mail`), requires a non-empty hash (hashing is auth's job — this
module only stores), defaults an empty role to `customer`, and rejects
anything that isn't `admin`/`customer`. All failures are coded `apperr`
errors. The `Repository` interface at the top is the key design move:
the service depends on an interface IT defines, so the pgx store and
test fakes are interchangeable. `var _ public.Service = (*service)(nil)`
makes the compiler prove the contract is honored.

### `handler/handler.go` — HTTP translation
Deliberately dumb: read param/body, call the service, write
`response.OK` / `response.Error`. An `if` about business rules here
would be a layering bug — logic belongs in `service`.

### `routes.go` — the route table
Maps URLs to handler methods, no logic: `GET /api/v1/users/:id` plus
the admin group `GET/PATCH /api/v1/admin/users/:id[/role]` wrapped in
`middleware.RequireRole("admin")`. Until the JWT middleware (auth
module) sets roles, `/admin/*` answers 401.

### `test/service_test.go` — proof without a database
A `fakeRepo` (two in-memory maps) stands in for Postgres. `TestCreate`
is table-driven across 7 cases (customer default, explicit admin kept,
email lowercasing, bad email, missing hash, bad role, duplicate email),
each asserting the exact `apperr.Code`. `TestGetAndSetRole` covers
case-insensitive lookup, promotion to admin, and the not-found paths.

## Related changes (outside this dir)

- `db/migrations/0002_user_roles.sql` — `role` column (default
  `'customer'`), check constraint, role index.
- `pkg/middleware/role.go` — `RequireRole` gate (401 no token, 403
  wrong role). Role key lives there, not in `auth`, to avoid an import
  cycle.
- `config/config.go` — `AdminEmails` (`ADMIN_EMAILS` env): the first
  admin can't be created via admin-only API, so auth's register will
  auto-grant `admin` to these emails.
- `cmd/api/main.go` — wires pool → repository → service → routes under
  `/api/v1`.

## Endpoints

| Method | Path                              | Access        |
|--------|-----------------------------------|---------------|
| GET    | `/api/v1/users/:id`               | public shape (auth-gated later) |
| GET    | `/api/v1/admin/users/:id`         | admin         |
| PATCH  | `/api/v1/admin/users/:id/role`    | admin, body `{"role": "admin"}` |

Interactive docs: `/swagger/index.html` (spec: `/swagger/doc.json`).
Regenerate after annotation changes:
`swag init -g cmd/api/main.go --parseInternal -o docs`.

## Single-storefront notes

- No `store_id` on any table; no sellers module.
- Admin manages the one store's users directly here.
- First admin: set `ADMIN_EMAILS` (or fallback SQL:
  `UPDATE users SET role='admin' WHERE email='you@shop.com';`).
