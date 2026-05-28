-- +goose Up
-- Budget Sentinel Insured Ship policies.
--
-- Premium is debited from the wallet at purchase time; payout
-- happens (via wallet topup) only when actual spend exceeds
-- hard_cap_usd inside the coverage window.

CREATE TABLE IF NOT EXISTS insurance_policies (
    id                     TEXT PRIMARY KEY,
    tenant_id              TEXT NOT NULL,
    project_id             TEXT NOT NULL,
    hard_cap_usd           NUMERIC(18, 6) NOT NULL CHECK (hard_cap_usd > 0),
    premium_usd            NUMERIC(18, 6) NOT NULL CHECK (premium_usd > 0),
    coverage_window_hours  INTEGER NOT NULL CHECK (coverage_window_hours > 0),
    status                 TEXT NOT NULL CHECK (status IN ('active', 'paid_out', 'expired', 'cancelled')),
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at             TIMESTAMPTZ NOT NULL,
    premium_op_key         TEXT NOT NULL,
    payout_op_key          TEXT
);

-- One active policy per project. Partial unique index so terminal
-- rows can coexist for audit history.
CREATE UNIQUE INDEX IF NOT EXISTS uq_insurance_active_per_project
    ON insurance_policies (tenant_id, project_id)
    WHERE status = 'active';

CREATE INDEX IF NOT EXISTS ix_insurance_tenant_created
    ON insurance_policies (tenant_id, created_at DESC);

CREATE INDEX IF NOT EXISTS ix_insurance_active_expires
    ON insurance_policies (expires_at)
    WHERE status = 'active';

-- +goose Down
DROP INDEX IF EXISTS ix_insurance_active_expires;
DROP INDEX IF EXISTS ix_insurance_tenant_created;
DROP INDEX IF EXISTS uq_insurance_active_per_project;
DROP TABLE IF EXISTS insurance_policies;
