-- 00038_user_roles.sql
-- Introduces a first-class role plane on the users table so audit /
-- operator gates can stop piggy-backing on the brittle Plan="operator"
-- shortcut. Roles is a string set (TEXT[]) with a GIN index so the
-- middleware can ask "does any user carry role X" cheaply.
--
-- Idempotent: every statement uses IF NOT EXISTS so re-running the
-- migration on an instance that has already been migrated is a no-op.

-- +goose Up
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS roles TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[];

CREATE INDEX IF NOT EXISTS idx_users_roles
    ON users USING GIN (roles);

-- Seed the platform_operator role on any account that was previously
-- marked as Plan='operator', so the transitional shortcut keeps working
-- and the role plane immediately reflects the existing operator set.
-- No-op when no such users exist yet.
UPDATE users
   SET roles = ARRAY['platform_operator']
 WHERE plan = 'operator'
   AND NOT (roles && ARRAY['platform_operator']);

-- +goose Down
DROP INDEX IF EXISTS idx_users_roles;
ALTER TABLE users DROP COLUMN IF EXISTS roles;
