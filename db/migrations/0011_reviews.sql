-- 0011: product reviews and ratings.
-- One review per user per product. Rating 1–5 enforced, not just hoped.
-- Product FK cascades (reviews die with the product); user FK cascades.
-- Purchase verification (only buyers may review) is out of scope for v1.
-- Applied with: psql "$DATABASE_URL" -f db/migrations/0011_reviews.sql

CREATE TABLE IF NOT EXISTS reviews (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    product_id UUID        NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    rating     INTEGER     NOT NULL CHECK (rating BETWEEN 1 AND 5),
    title      TEXT        NOT NULL DEFAULT '',
    body       TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, product_id)
);

CREATE INDEX IF NOT EXISTS idx_reviews_product ON reviews (product_id);
