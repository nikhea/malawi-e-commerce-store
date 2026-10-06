-- 0007: stock levels + order reservations.
-- One row per product/variant (NULL variant = base product stock).
-- reserved_qty can never exceed on_hand_qty (enforced, not just hoped).
-- Reservations ledger: active holds become confirmed (sold) or released.
-- Expiry sweeping belongs to the worker (orders passes a TTL at reserve).
-- Applied with: psql "$DATABASE_URL" -f db/migrations/0007_inventory.sql

CREATE TABLE IF NOT EXISTS inventory (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id    UUID        NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    variant_id    UUID        REFERENCES variants (id) ON DELETE CASCADE,
    on_hand_qty   INTEGER     NOT NULL DEFAULT 0 CHECK (on_hand_qty >= 0),
    reserved_qty  INTEGER     NOT NULL DEFAULT 0 CHECK (reserved_qty >= 0),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (reserved_qty <= on_hand_qty)
);

-- One row per product/variant (NULL variant collapses to zero UUID,
-- same trick as the cart line index).
CREATE UNIQUE INDEX IF NOT EXISTS idx_inventory_unique_stock
    ON inventory (product_id, COALESCE(variant_id, '00000000-0000-0000-0000-000000000000'));

CREATE TABLE IF NOT EXISTS reservations (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id  UUID        NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    variant_id  UUID        REFERENCES variants (id) ON DELETE CASCADE,
    qty         INTEGER     NOT NULL CHECK (qty > 0),
    order_ref   TEXT        NOT NULL,
    status      TEXT        NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'confirmed', 'released')),
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_reservations_order ON reservations (order_ref);
CREATE INDEX IF NOT EXISTS idx_reservations_expiry ON reservations (expires_at) WHERE status = 'active';
