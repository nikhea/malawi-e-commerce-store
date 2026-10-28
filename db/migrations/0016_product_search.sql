-- 0016: PostgreSQL full-text search for the product catalog.
--
-- search_vector is a generated column, so it can never drift from the row.
-- Name and description use English stemming; the slug uses the simple
-- dictionary after turning hyphens into spaces. Weights feed ts_rank_cd:
-- A = name, B = description, C = slug.
--
-- The GIN index serves `search_vector @@ to_tsquery(...)`. The trigram
-- indexes from 0014 stay in place for the substring fallback used when a
-- query has no full-text matches.
--
-- NOTE: adding a stored generated column rewrites products under an
-- ACCESS EXCLUSIVE lock. Fine at this catalog size; for a much larger
-- table, add a nullable tsvector, backfill in batches, then index it.
--
-- Applied with: psql "$DATABASE_URL" -f db/migrations/0016_product_search.sql

ALTER TABLE products
    ADD COLUMN IF NOT EXISTS search_vector tsvector
    GENERATED ALWAYS AS (
        setweight(to_tsvector('english', coalesce(name, '')), 'A') ||
        setweight(to_tsvector('english', coalesce(description, '')), 'B') ||
        setweight(to_tsvector('simple', replace(coalesce(slug, ''), '-', ' ')), 'C')
    ) STORED;

CREATE INDEX IF NOT EXISTS idx_products_search_vector
    ON products USING gin (search_vector);

ANALYZE products;
