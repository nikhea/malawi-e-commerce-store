-- 0012: author snapshot on reviews.
-- 0011 shipped without the author column the repository inserts.
-- (Caught live: review writes 500'd on the missing column.) Backfill
-- from users.name where possible; fall back to '' (service prefers
-- email when the name is blank anyway).
-- Applied with: psql "$DATABASE_URL" -f db/migrations/0012_review_author.sql

ALTER TABLE reviews ADD COLUMN IF NOT EXISTS author TEXT NOT NULL DEFAULT '';

UPDATE reviews r
SET author = COALESCE(NULLIF(u.name, ''), u.email, '')
FROM users u
WHERE u.id = r.user_id AND (r.author IS NULL OR r.author = '');
