-- 0004: optional category images (Cloudinary via River queue).
-- Image columns stay empty-string default: images are OPTIONAL, listing
-- and detail reads work identically with or without them.
-- Applied with: psql "$DATABASE_URL" -f db/migrations/0004_category_images.sql

ALTER TABLE categories ADD COLUMN IF NOT EXISTS image_url TEXT NOT NULL DEFAULT '';
ALTER TABLE categories ADD COLUMN IF NOT EXISTS image_public_id TEXT NOT NULL DEFAULT '';
