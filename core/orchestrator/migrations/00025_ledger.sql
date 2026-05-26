-- +goose Up
-- +goose StatementBegin
--
-- V22 ledger (Agent 3) — append-only financial ledger that proves
-- every dollar in / dollar out for every paid execution. Schema is
-- adapted from
-- docs/ironflyer_deep_atomic_plan_v22_profit_scale_proof_pack/
--   05-billing-ledger/02-ledger-schema.sql
-- with two deliberate deviations:
--
--   * tenant_id / execution_id are plain UUIDs, NOT foreign-key
--     references. The historical "tenants" table does not exist as a
--     separate entity in this codebase (users IS the tenant), and
--     "executions" is created by migration 00026 (Agent 4) — adding a
--     hard FK here would introduce a circular ordering risk between
--     parallel agents.
--
--   * uuid generation uses pgcrypto's gen_random_uuid() instead of
--     uuid_generate_v4(); pgcrypto ships with Postgres core whereas
--     uuid-ossp is an extra extension we don't otherwise require.
--
-- The ledger is append-only by contract — the Go service surface in
-- internal/ledger/ exposes Write + read operations only. Refunds and
-- credit releases are new entries, never UPDATEs.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS ledger_entries (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
    execution_id    UUID,
    entry_type      TEXT NOT NULL CHECK (entry_type IN (
        'wallet_topup',
        'credit_reservation',
        'provider_inference_cost',
        'sandbox_cost',
        'storage_cost',
        'deployment_cost',
        'refund',
        'credit_release',
        'platform_margin',
        'premium_reasoning_charge'
    )),
    direction       TEXT NOT NULL CHECK (direction IN ('debit', 'credit')),
    amount_usd      NUMERIC(18, 6) NOT NULL CHECK (amount_usd > 0),
    provider        TEXT,
    billable        BOOLEAN NOT NULL DEFAULT true,
    margin_relevant BOOLEAN NOT NULL DEFAULT true,
    metadata        JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ledger_tenant_created
    ON ledger_entries(tenant_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ledger_execution
    ON ledger_entries(execution_id);

CREATE INDEX IF NOT EXISTS idx_ledger_type_created
    ON ledger_entries(entry_type, created_at DESC);

-- Dashboard rollups (Profit / Cohort) scan a tenant's entries grouped
-- by entry_type within a time window — this composite index keeps the
-- rollup queries off a full table scan as ledger volume grows.
CREATE INDEX IF NOT EXISTS idx_ledger_tenant_type_created
    ON ledger_entries(tenant_id, entry_type, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS ledger_entries;
-- +goose StatementEnd
