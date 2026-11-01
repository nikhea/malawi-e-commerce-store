-- 0018: product gallery images (multiple per product).
-- products.image_url / products.image_public_id stay the PRIMARY image for
-- backward compatibility; product_images rows are the gallery. url and
-- public_id start empty while the River upload job runs, then the media
-- completion callback fills them in. Pending rows are hidden from product
-- detail until they have a url.
-- Applied with: psql "$DATABASE_URL" -f db/migrations/0018_product_images.sql

CREATE TABLE IF NOT EXISTS product_images (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id  UUID        NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    url         TEXT        NOT NULL DEFAULT '',
    public_id   TEXT        NOT NULL DEFAULT '',
    position    INTEGER     NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_product_images_product
    ON product_images (product_id, position, created_at);
