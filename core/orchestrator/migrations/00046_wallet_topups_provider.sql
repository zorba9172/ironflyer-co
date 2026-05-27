-- +goose Up
-- +goose StatementBegin
-- Multi-provider wallet top-ups. The wallet_topups table predates
-- Paddle; its stripe_session_id column now stores any provider's
-- transaction reference (Stripe cs_*, Paddle txn_*). Adding an
-- explicit provider column lets the reconciliation cron and the
-- profit dashboards split top-up volume per PSP without parsing the
-- id prefix.
--
-- Backfill rule: existing rows are Stripe — wallet_topups did not
-- exist before V22 and Stripe was the only writer between V22 and
-- this migration.
ALTER TABLE wallet_topups
    ADD COLUMN IF NOT EXISTS provider TEXT NOT NULL DEFAULT 'stripe'
        CHECK (provider IN ('stripe','paddle'));
-- +goose StatementEnd

-- +goose StatementBegin
-- Index for the reconciliation cron, which sweeps top-ups per
-- provider over the last 48h and asserts every paid vendor event
-- has a corresponding 'succeeded' row.
CREATE INDEX IF NOT EXISTS idx_wallet_topups_provider_created
    ON wallet_topups(provider, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_wallet_topups_provider_created;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE wallet_topups DROP COLUMN IF EXISTS provider;
-- +goose StatementEnd
