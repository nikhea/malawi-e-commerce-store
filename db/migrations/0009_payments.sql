-- 0009: Stripe payments ledger.
-- One row per order (UNIQUE): retries reuse the intent instead of
-- double-charging. amount_cents is the CHARGED amount (USD cents);
-- order_amount_cents preserves the source MWK tambala for reconciliation.
-- Webhook retries are safe because order transitions are idempotent —
-- no separate processed-events table needed (documented, not forgotten).
-- Applied with: psql "$DATABASE_URL" -f db/migrations/0009_payments.sql

CREATE TABLE IF NOT EXISTS payments (
    id                 UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id           UUID        NOT NULL UNIQUE REFERENCES orders (id) ON DELETE CASCADE,
    stripe_intent_id   TEXT        NOT NULL UNIQUE,
    client_secret      TEXT        NOT NULL DEFAULT '',
    amount_cents       BIGINT      NOT NULL CHECK (amount_cents >= 0),
    currency           TEXT        NOT NULL DEFAULT 'USD',
    order_amount_cents BIGINT      NOT NULL DEFAULT 0,
    status             TEXT        NOT NULL DEFAULT 'pending'
                                   CHECK (status IN ('pending', 'succeeded', 'failed', 'cancelled')),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_payments_intent ON payments (stripe_intent_id);
