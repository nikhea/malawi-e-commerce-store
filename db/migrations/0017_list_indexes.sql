-- 0017: indexes backing the paged shopper lists.
-- Cart lines, order history, wishlist hearts and product reviews are all
-- read one page at a time and sorted by age. These composite indexes keep
-- the COUNT + page pair from falling back to a sort of every row.
-- Applied with: psql "$DATABASE_URL" -f db/migrations/0017_list_indexes.sql

CREATE INDEX IF NOT EXISTS idx_cart_items_cart_created
    ON cart_items (cart_id, created_at, id);

CREATE INDEX IF NOT EXISTS idx_orders_user_created
    ON orders (user_id, created_at DESC, id);

CREATE INDEX IF NOT EXISTS idx_wishlist_user_created
    ON wishlist_items (user_id, created_at DESC, id);

CREATE INDEX IF NOT EXISTS idx_reviews_product_created
    ON reviews (product_id, created_at DESC, id);
