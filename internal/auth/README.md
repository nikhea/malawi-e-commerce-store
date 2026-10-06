# auth module

JWT authentication for the single storefront: register, login, and token
verification. Depends on `users/public` (and nothing else); `pkg/middleware`
depends on `auth/public` for the JWT middleware. No new tables — accounts
live in `users`, auth only hashes passwords and mints tokens.

Layout follows AGENTS.md §2. For the layer-by-layer file guide see
`internal/users/README.md`; this file covers how to READ the auth flow.

## Files

- `public/api.go` — contract: `Service` (Register/Login/Parse),
  `Claims`, `TokenPair` (flat scalars — see note), `RegisterInput`.
- `utils/password.go` — bcrypt `Hash`/`Verify` (constant-time compare).
- `utils/jwt.go` — HS256 `Sign`/`Parse`. Rejects non-HS256 algorithms
  (alg-confusion) and non-admin/customer role claims.
- `service/service.go` — orchestration. Constructor takes
  `userspublic.Service` + `Config{Secret, TTL, AdminEmails}`. Register:
  validate → hash → role from `AdminEmails` → `users.Create`.
  Login: `GetCredentials` → verify → issue. Both login failures return
  identical `invalid credentials` (no account enumeration).
- `dto/request.go` — `RegisterRequest` / `LoginRequest` bodies.
- `handler/handler.go` — thin register/login handlers + swag annotations.
- `routes.go` — open group (`/auth/register`, `/auth/login`).
- `test/service_test.go` — fake users service; password roundtrip, JWT
  tampering, register/login matrices.

## Note: TokenPair is flat, not nested

`TokenPair` carries `user_id/email/name/role` as scalars instead of a
nested `users.User`. Reason: swag cannot resolve types nested across two
`public` packages, so generated docs break. Rule for future modules:
swagger-documented DTOs must not nest types from another module's
`public/` package — use scalar wire types. (Go code keeps strong typing
everywhere else; `Claims.Role` is still `userspublic.Role`.)

## How to trace a request (two paths, opposite directions)

### Path 1 — Startup: read `cmd/api/main.go:37-81` bottom-up

Ordered from "knows nothing" to "knows everything": config → pool →
users repo → users service → auth service (note `usersSvc` injected —
the cross-module seam) → route groups. A constructor's arguments are
always built above it.

### Path 2 — Request: trace top-down (`POST /auth/register`)

1. `internal/auth/routes.go` — the map (URL → handler). Start here.
2. `internal/auth/dto/request.go` — the incoming JSON shape.
3. `internal/auth/handler/handler.go` — bind dto → service → envelope.
4. `internal/auth/public/api.go` — what the service promises.
5. `internal/auth/service/service.go` — validate → hash → users.Create.
6. `internal/auth/utils/*.go` — pure tools, no app imports.
7. `internal/users/…` — same layers again (contract → repo → SQL).
8. `pkg/response` — envelope on the way out.

Chain: routes → dto → handler → public → service → utils →
(sibling public → service → repository → db) → response.

### Authenticated requests: the middleware prelude

`GET /api/v1/users/me` runs this before step 1: `protected` group
(`main.go:79-81`) → `pkg/middleware/jwt.go` (strip `Bearer `, Parse,
`SetRole` + `user_id`) → `pkg/middleware/role.go` on `/admin/*` only
(401 vs 403) → users `Me` handler reads `middleware.UserIDOf(c)`.
Identity comes from the token, never a URL param — clients can't spoof
other users.

### Universal technique: follow the imports

Each file's import block is a "read next" list. For callers, grep the
function name. To confirm no import cycle, check that `pkg/middleware`
imports only `auth/public` (contract), never `auth/service`.

## Endpoints

| Method | Path                      | Access | Notes                          |
|--------|---------------------------|--------|--------------------------------|
| POST   | `/api/v1/auth/register`   | open   | 201 + token; 409 duplicate     |
| POST   | `/api/v1/auth/login`      | open   | 200 + token; 401 stays vague  |

Interactive docs: `/swagger/index.html`.
Admin bootstrap: `ADMIN_EMAILS` env (auto-admin at register), or
`UPDATE users SET role='admin' WHERE email='…';`.
