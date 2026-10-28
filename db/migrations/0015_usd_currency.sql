-- 0015: switch the storefront to USD.
--
-- Products, variants and live carts move from MWK tambala to USD cents.
-- Historical orders keep their original amounts: an order is the legal
-- record of what the shopper agreed to pay, and payments convert legacy
-- MWK orders at charge time.
--
-- Conversion is value-preserving at 1 USD = 1,700 MWK. ROUND keeps money
-- in integer minor units — never floats at rest.
--
-- Applied with: psql "$DATABASE_URL" -f db/migrations/0015_usd_currency.sql

BEGIN;

-- Variants inherit their product's currency (no currency column of their
-- own), so convert them before products drop the MWK marker.
UPDATE variants v
SET price_cents = ROUND(v.price_cents / 1700.0)::BIGINT
FROM products p
WHERE v.product_id = p.id
  AND p.currency = 'MWK';

UPDATE products
SET price_cents = ROUND(price_cents / 1700.0)::BIGINT,
    currency = 'USD'
WHERE currency = 'MWK';

UPDATE cart_items
SET unit_price_cents = ROUND(unit_price_cents / 1700.0)::BIGINT,
    currency = 'USD'
WHERE currency = 'MWK';

ALTER TABLE products ALTER COLUMN currency SET DEFAULT 'USD';
ALTER TABLE cart_items ALTER COLUMN currency SET DEFAULT 'USD';
ALTER TABLE orders ALTER COLUMN currency SET DEFAULT 'USD';

COMMIT;
