-- 0001: identity foundation (users table).
-- Applied with: psql "$DATABASE_URL" -f db/migrations/0001_users.sql
-- Email is stored lowercased by the service; the UNIQUE constraint then
-- behaves case-insensitively without needing the citext extension.

CREATE TABLE IF NOT EXISTS users (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT        NOT NULL UNIQUE,
    password_hash TEXT        NOT NULL,
    name          TEXT        NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users (email);
