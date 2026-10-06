-- 0006: shopping carts (one active cart per user).
-- Prices are snapshotted into cart_items at add-time: catalog prices move,
-- carts must not. Variant optional (simple products have no SKUs).
-- Product/variant FKs CASCADE: a deleted product takes its cart lines
-- with it instead of breaking checkout reads.
-- Applied with: psql "$DATABASE_URL" -f db/migrations/0006_cart.sql

CREATE TABLE IF NOT EXISTS carts (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL UNIQUE REFERENCES users (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS cart_items (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    cart_id          UUID        NOT NULL REFERENCES carts (id) ON DELETE CASCADE,
    product_id       UUID        NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    variant_id       UUID        REFERENCES variants (id) ON DELETE CASCADE,
    name             TEXT        NOT NULL DEFAULT '',
    sku              TEXT        NOT NULL DEFAULT '',
    unit_price_cents BIGINT      NOT NULL DEFAULT 0 CHECK (unit_price_cents >= 0),
    currency         TEXT        NOT NULL DEFAULT 'MWK',
    qty              INTEGER     NOT NULL DEFAULT 1 CHECK (qty > 0),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_cart_items_cart ON cart_items (cart_id);

-- One line per product/variant combo. Plain UNIQUE would treat NULL
-- variant_ids as distinct (allowing duplicates), so COALESCE the null
-- into the zero UUID instead.
CREATE UNIQUE INDEX IF NOT EXISTS idx_cart_items_unique_line
    ON cart_items (cart_id, product_id, COALESCE(variant_id, '00000000-0000-0000-0000-000000000000'));
