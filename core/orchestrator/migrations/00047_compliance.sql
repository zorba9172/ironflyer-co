-- +goose Up
-- +goose StatementBegin
-- ComplianceGate verticals: PCI / HIPAA / SOC 2 / GDPR enrolments,
-- per-control evaluation history, and the monthly subscription
-- charges that bill them through the wallet. Backed by the
-- internal/business/compliance package.
--
-- One enrolment per (tenant, project, framework). The unique
-- constraint is what turns the resolver's "double-click enroll"
-- into a no-op without an extra round trip.
CREATE TABLE IF NOT EXISTS compliance_enrollments (
    id                TEXT        PRIMARY KEY,
    tenant_id         TEXT        NOT NULL,
    project_id        TEXT        NOT NULL,
    framework_key     TEXT        NOT NULL,
    enrolled_at       TIMESTAMPTZ NOT NULL,
    last_evaluated_at TIMESTAMPTZ,
    last_verdict      TEXT        NOT NULL DEFAULT 'pending'
                       CHECK (last_verdict IN ('pending','pass','fail')),
    next_charge_at    TIMESTAMPTZ NOT NULL,
    UNIQUE (tenant_id, project_id, framework_key)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_compliance_enrollments_tenant
    ON compliance_enrollments (tenant_id, enrolled_at DESC);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_compliance_enrollments_next_charge
    ON compliance_enrollments (next_charge_at);
-- +goose StatementEnd

-- +goose StatementBegin
-- One row per control finding from the latest EvaluateAll. SaveResults
-- atomically replaces the per-enrolment slice so the dashboard always
-- shows the freshest verdict; older runs are not retained here (the
-- audit bundle export carries the snapshot if the operator needs it).
CREATE TABLE IF NOT EXISTS compliance_results (
    id            TEXT        PRIMARY KEY,
    enrollment_id TEXT        NOT NULL REFERENCES compliance_enrollments(id) ON DELETE CASCADE,
    control_key   TEXT        NOT NULL,
    framework_key TEXT        NOT NULL,
    status        TEXT        NOT NULL
                   CHECK (status IN ('pass','fail','n/a')),
    severity      TEXT        NOT NULL,
    evidence      TEXT        NOT NULL DEFAULT '',
    path          TEXT,
    evaluated_at  TIMESTAMPTZ NOT NULL
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_compliance_results_enrollment
    ON compliance_results (enrollment_id, evaluated_at DESC);
-- +goose StatementEnd

-- +goose StatementBegin
-- Monthly subscription debits. idempotency_key is the
-- "compliance:<tenant>:<project>:<framework>:<YYYY-MM>" string the
-- service generates, so a re-tick during the same calendar month
-- collapses to a no-op via the UNIQUE constraint.
CREATE TABLE IF NOT EXISTS compliance_charges (
    id               TEXT          PRIMARY KEY,
    enrollment_id    TEXT          NOT NULL REFERENCES compliance_enrollments(id) ON DELETE CASCADE,
    tenant_id        TEXT          NOT NULL,
    framework_key    TEXT          NOT NULL,
    period           TEXT          NOT NULL,
    amount_usd       NUMERIC(20,6) NOT NULL CHECK (amount_usd >= 0),
    charged_at       TIMESTAMPTZ   NOT NULL,
    idempotency_key  TEXT          NOT NULL UNIQUE
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_compliance_charges_tenant_period
    ON compliance_charges (tenant_id, period DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS compliance_charges;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS compliance_results;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS compliance_enrollments;
-- +goose StatementEnd
