-- +goose Up
-- +goose StatementBegin
--
-- V22 purge: drop every table that backed packages removed in the
-- V22 architecture overhaul. Idempotent: every drop uses IF EXISTS
-- and CASCADE to detach any leftover foreign keys without erroring
-- when run against a fresh database that never carried these tables.
--
-- See docs/V22_PLAN.md for the package removal list. The new V22
-- domain (wallets, ledger_entries, executions, blueprints, repair
-- recipes, profit_guard_decisions) is created by migrations
-- 00024-00029 owned by their respective agents.

-- Leads (00003_init_leads.sql)
DROP TABLE IF EXISTS leads CASCADE;

-- Integrations / GitHub tokens (00004_init_integrations.sql)
DROP TABLE IF EXISTS integration_tokens CASCADE;
DROP TABLE IF EXISTS webhook_deliveries CASCADE;

-- PR reviews (00005_init_prreview.sql)
DROP TABLE IF EXISTS pr_reviews CASCADE;
DROP TABLE IF EXISTS pr_review_runs CASCADE;
DROP TABLE IF EXISTS pr_review_comments CASCADE;

-- Memory federation (00006_init_memory_federation.sql)
DROP TABLE IF EXISTS memory_federation_members CASCADE;
DROP TABLE IF EXISTS memory_federation_groups CASCADE;

-- Affiliates (00007_init_affiliates.sql)
DROP TABLE IF EXISTS affiliates CASCADE;
DROP TABLE IF EXISTS affiliate_referrals CASCADE;
DROP TABLE IF EXISTS affiliate_payouts CASCADE;

-- Custom domains (00008_init_domains.sql)
DROP TABLE IF EXISTS custom_domains CASCADE;

-- Chats (00009_init_chats.sql)
DROP TABLE IF EXISTS chat_messages CASCADE;
DROP TABLE IF EXISTS chats CASCADE;

-- Share links (00010_init_sharelinks.sql)
DROP TABLE IF EXISTS share_links CASCADE;

-- Collab (00011_init_collab.sql)
DROP TABLE IF EXISTS collaborators CASCADE;

-- Auth SAML (00012_init_auth_saml.sql)
DROP TABLE IF EXISTS auth_saml_configs CASCADE;

-- Auth IP allowlist (00013_init_auth_ipallowlist.sql)
DROP TABLE IF EXISTS auth_ip_allowlists CASCADE;

-- Demo seed (00014_init_auth_demo_seed.sql)
-- The demo seed populated rows in users + projects + share_links.
-- Removing the seed row keeps the demo user (still useful for dev login)
-- intact but drops any seeded share-link rows. The share_links table itself
-- is already dropped above. No additional action required here.

-- Dunning (00019_dunning_states.sql)
DROP TABLE IF EXISTS dunning_states CASCADE;
DROP TABLE IF EXISTS dunning_attempts CASCADE;

-- MFA (00022_mfa.sql)
DROP TABLE IF EXISTS auth_mfa_factors CASCADE;
DROP TABLE IF EXISTS auth_mfa_challenges CASCADE;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- V22 purge is intentionally one-way. Re-introducing these tables would
-- require restoring the packages they backed, which is not the V22 model.
-- This Down block is a no-op so accidentally rolling back the migration
-- does not error, but it cannot recreate the dropped tables.
SELECT 1;
-- +goose StatementEnd
