-- 0002: admin roles for the single storefront.
-- One store, no sellers: access control is a role flag on users, not a
-- separate tenants/sellers model. Applied with:
--   psql "$DATABASE_URL" -f db/migrations/0002_user_roles.sql

ALTER TABLE users ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'customer';

DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'users_role_check') THEN
    ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('admin', 'customer'));
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_users_role ON users (role);
