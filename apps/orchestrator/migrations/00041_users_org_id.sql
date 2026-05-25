-- +goose Up
-- 00041_users_org_id — add the org_id column the auth.PostgresUserStore
-- has been COALESCEing for ages. Without it, GetByID / GetByEmail blow
-- up with `column "org_id" does not exist` the first time a Bearer
-- token is verified, which silently fails through auth.Optional and
-- the policy plane sees `principal.kind == "anonymous"` for every
-- authenticated GraphQL call.
--
-- SAML bootstrap (BootstrapSAMLPostgres) and admin tooling (SetOrg)
-- write to this column too; we keep it nullable + indexed because
-- single-tenant accounts legitimately have no org membership and the
-- lookup pattern is WHERE LOWER(email) = $1, not WHERE org_id = $1.

ALTER TABLE users ADD COLUMN IF NOT EXISTS org_id TEXT;

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS org_id;
