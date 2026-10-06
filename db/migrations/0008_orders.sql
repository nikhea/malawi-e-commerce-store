-- 0008: checkout state machine (orders + lines).
-- Prices snapshotted AGAIN from the cart: the order is the legal record,
-- immune to later catalog or cart changes. Status flows pending →
-- paid | cancelled, enforced by CHECK + guarded transitions in the repo.
-- Applied with: psql "$DATABASE_URL" -f db/migrations/0008_orders.sql

CREATE TABLE IF NOT EXISTS orders (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    status         TEXT        NOT NULL DEFAULT 'pending'
                               CHECK (status IN ('pending', 'paid', 'cancelled')),
    subtotal_cents BIGINT      NOT NULL DEFAULT 0 CHECK (subtotal_cents >= 0),
    currency       TEXT        NOT NULL DEFAULT 'MWK',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_orders_user ON orders (user_id);
CREATE INDEX IF NOT EXISTS idx_orders_status ON orders (status) WHERE status = 'pending';

CREATE TABLE IF NOT EXISTS order_items (
    id               UUID   PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id         UUID   NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    product_id       UUID   NOT NULL,
    variant_id       UUID,
    name             TEXT   NOT NULL DEFAULT '',
    sku              TEXT   NOT NULL DEFAULT '',
    unit_price_cents BIGINT NOT NULL DEFAULT 0 CHECK (unit_price_cents >= 0),
    qty              INTEGER NOT NULL CHECK (qty > 0)
);

CREATE INDEX IF NOT EXISTS idx_order_items_order ON order_items (order_id);
