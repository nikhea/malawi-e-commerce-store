-- 0005: catalog core (products + SKUs).
-- Money is BIGINT minor units (tambala): never floats for prices.
-- products.category_id SET NULL: deleting a category unfiles products
-- instead of deleting the catalog. variants cascade: a product's SKUs
-- die with it. SKU globally unique (barcodes don't repeat).
-- Applied with: psql "$DATABASE_URL" -f db/migrations/0005_products.sql

CREATE TABLE IF NOT EXISTS products (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id     UUID        REFERENCES categories (id) ON DELETE SET NULL,
    name            TEXT        NOT NULL,
    slug            TEXT        NOT NULL UNIQUE,
    description     TEXT        NOT NULL DEFAULT '',
    price_cents     BIGINT      NOT NULL DEFAULT 0 CHECK (price_cents >= 0),
    currency        TEXT        NOT NULL DEFAULT 'MWK',
    image_url       TEXT        NOT NULL DEFAULT '',
    image_public_id TEXT        NOT NULL DEFAULT '',
    is_active       BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_products_category ON products (category_id);
CREATE INDEX IF NOT EXISTS idx_products_active ON products (is_active) WHERE is_active;

CREATE TABLE IF NOT EXISTS variants (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id  UUID        NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    name        TEXT        NOT NULL,
    sku         TEXT        NOT NULL UNIQUE,
    price_cents BIGINT      NOT NULL DEFAULT 0 CHECK (price_cents >= 0),
    is_active   BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_variants_product ON variants (product_id);
