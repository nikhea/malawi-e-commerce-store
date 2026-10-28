-- 0014: indexes for catalog search, price/date sorts and stock filters.
-- pg_trgm is trusted from PG13 onward, so the database owner can install
-- it without superuser. Trigram GIN indexes make ILIKE '%term%' usable
-- instead of a sequential scan as the catalog grows.
-- Applied with: psql "$DATABASE_URL" -f db/migrations/0014_catalog_filters.sql

CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Substring search over the public catalog fields.
CREATE INDEX IF NOT EXISTS idx_products_name_trgm
    ON products USING gin (lower(name) gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_products_description_trgm
    ON products USING gin (lower(description) gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_products_slug_trgm
    ON products USING gin (lower(slug) gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_categories_name_trgm
    ON categories USING gin (lower(name) gin_trgm_ops);

-- Sort and range filters.
CREATE INDEX IF NOT EXISTS idx_products_price_active
    ON products (price_cents) WHERE is_active;
CREATE INDEX IF NOT EXISTS idx_products_created_at
    ON products (created_at DESC);

-- `in_stock=true` checks availability per product.
CREATE INDEX IF NOT EXISTS idx_inventory_in_stock
    ON inventory (product_id) WHERE on_hand_qty > reserved_qty;

-- Size filter: one indexed probe per requested size.
CREATE INDEX IF NOT EXISTS idx_variants_product_size
    ON variants (product_id, lower(name)) WHERE is_active;

-- Rating filter/order reads product_id + rating only.
CREATE INDEX IF NOT EXISTS idx_reviews_product_rating
    ON reviews (product_id, rating);
