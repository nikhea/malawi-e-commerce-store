-- 0010: wishlist links (user → product).
-- One row per user/product (re-hearting is a no-op, not a duplicate).
-- Product FK cascades: deleted products vanish from wishlists silently.
-- Applied with: psql "$DATABASE_URL" -f db/migrations/0010_wishlist.sql

CREATE TABLE IF NOT EXISTS wishlist_items (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    product_id UUID        NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, product_id)
);

CREATE INDEX IF NOT EXISTS idx_wishlist_user ON wishlist_items (user_id);
