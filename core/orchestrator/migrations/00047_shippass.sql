-- +goose Up
-- Ship Pass — outcome-based SKU on top of the wallet.
--
-- A pass is an atomic promise: hold the price until every gate in
-- the tier scope passes (debit) or the deadline elapses (release).
-- The wallet hold lives in the existing wallet tables; this schema
-- just tracks the SKU lifecycle.

CREATE TABLE IF NOT EXISTS ship_passes (
    id              TEXT PRIMARY KEY,
    tenant_id       TEXT NOT NULL,
    project_id      TEXT NOT NULL,
    tier_key        TEXT NOT NULL,
    price_usd       NUMERIC(18, 6) NOT NULL CHECK (price_usd > 0),
    status          TEXT NOT NULL CHECK (status IN ('active', 'shipped', 'refunded', 'cancelled')),
    deadline_at     TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    settled_at      TIMESTAMPTZ,
    hold_op_key     TEXT NOT NULL,
    debit_op_key    TEXT,
    refund_op_key   TEXT
);

-- One active pass per project. Partial index so the unique constraint
-- only applies to in-flight rows; terminal rows pile up without
-- collision.
CREATE UNIQUE INDEX IF NOT EXISTS uq_ship_passes_active_per_project
    ON ship_passes (tenant_id, project_id)
    WHERE status = 'active';

CREATE INDEX IF NOT EXISTS ix_ship_passes_tenant_created
    ON ship_passes (tenant_id, created_at DESC);

CREATE INDEX IF NOT EXISTS ix_ship_passes_active_due
    ON ship_passes (deadline_at)
    WHERE status = 'active';

-- Per-pass gate verdict observation log. Every verdict (not just the
-- latest) is stored so the Feedback Brain can mine "almost shipped"
-- cohorts. The Settler reduces to latest-per-gate at decision time.
CREATE TABLE IF NOT EXISTS ship_pass_progress (
    id              TEXT PRIMARY KEY,
    ship_pass_id    TEXT NOT NULL REFERENCES ship_passes(id) ON DELETE CASCADE,
    gate            TEXT NOT NULL,
    passed          BOOLEAN NOT NULL,
    reason          TEXT,
    observed_at     TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS ix_ship_pass_progress_pass_observed
    ON ship_pass_progress (ship_pass_id, observed_at DESC);

-- +goose Down
DROP INDEX IF EXISTS ix_ship_pass_progress_pass_observed;
DROP TABLE IF EXISTS ship_pass_progress;
DROP INDEX IF EXISTS ix_ship_passes_active_due;
DROP INDEX IF EXISTS ix_ship_passes_tenant_created;
DROP INDEX IF EXISTS uq_ship_passes_active_per_project;
DROP TABLE IF EXISTS ship_passes;
