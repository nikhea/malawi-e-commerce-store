-- 0003: catalog taxonomy (categories tree).
-- Single storefront: one shared tree, no store_id. Children of a deleted
-- category become roots (SET NULL) instead of vanishing from the catalog.
-- Applied with: psql "$DATABASE_URL" -f db/migrations/0003_categories.sql

CREATE TABLE IF NOT EXISTS categories (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT        NOT NULL,
    slug        TEXT        NOT NULL UNIQUE,
    description TEXT        NOT NULL DEFAULT '',
    parent_id   UUID        REFERENCES categories (id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_categories_parent ON categories (parent_id);
