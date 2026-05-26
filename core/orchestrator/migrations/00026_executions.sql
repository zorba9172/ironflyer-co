-- +goose Up
-- +goose StatementBegin
-- V22 execution entity — every paid run of the finisher is tracked as a
-- single measured economic unit. ProfitGuard reads the live numeric
-- columns through Service.GetState() before every expensive call; the
-- finisher updates spent_usd / completion_score as work progresses;
-- the dashboards aggregate over rows by status + window.
--
-- Money columns are NUMERIC(18,6) USD (matches the wallet schema).
-- completion_score is a 0..1 fraction; gross_margin_pct is a percentage
-- (0..100), nullable until revenue is recorded so the dashboards can
-- distinguish "no revenue yet" from "0% margin".
CREATE TYPE execution_status AS ENUM (
    'created',
    'admitted',
    'running',
    'paused_for_budget',
    'succeeded',
    'failed',
    'stopped',
    'killed',
    'refunded'
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS executions (
    id                        UUID             PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id                 UUID             NOT NULL,
    project_id                UUID,
    blueprint_id              TEXT,
    status                    execution_status NOT NULL DEFAULT 'created',
    budget_usd                NUMERIC(18,6)    NOT NULL,
    reserved_usd              NUMERIC(18,6)    NOT NULL DEFAULT 0,
    spent_usd                 NUMERIC(18,6)    NOT NULL DEFAULT 0,
    refunded_usd              NUMERIC(18,6)    NOT NULL DEFAULT 0,
    revenue_usd               NUMERIC(18,6)    NOT NULL DEFAULT 0,
    provider_cost_usd         NUMERIC(18,6)    NOT NULL DEFAULT 0,
    sandbox_cost_usd          NUMERIC(18,6)    NOT NULL DEFAULT 0,
    storage_cost_usd          NUMERIC(18,6)    NOT NULL DEFAULT 0,
    deployment_cost_usd       NUMERIC(18,6)    NOT NULL DEFAULT 0,
    completion_score          NUMERIC(5,4)     NOT NULL DEFAULT 0,
    completion_score_initial  NUMERIC(5,4)     NOT NULL DEFAULT 0,
    gross_margin_pct          NUMERIC(7,4),
    expected_completion_delta NUMERIC(5,4),
    risk_score                NUMERIC(5,4),
    stop_loss_usd             NUMERIC(18,6),
    prompt_summary            TEXT,
    failure_reason            TEXT,
    metadata                  JSONB            NOT NULL DEFAULT '{}'::jsonb,
    created_at                TIMESTAMPTZ      NOT NULL DEFAULT now(),
    admitted_at               TIMESTAMPTZ,
    started_at                TIMESTAMPTZ,
    ended_at                  TIMESTAMPTZ,
    CHECK (budget_usd > 0),
    CHECK (spent_usd >= 0),
    CHECK (refunded_usd >= 0),
    CHECK (completion_score BETWEEN 0 AND 1)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_executions_tenant
    ON executions(tenant_id, created_at DESC);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_executions_status
    ON executions(status, created_at DESC);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_executions_blueprint
    ON executions(blueprint_id, created_at DESC);
-- +goose StatementEnd

-- +goose StatementBegin
-- execution_events is the append-only audit / progress feed for one
-- execution. Subscribers (GraphQL executionFeed subscription) read this
-- table via Postgres LISTEN/NOTIFY on the 'execution_events' channel;
-- the payload column carries the structured event body (cost amount,
-- score delta, ProfitGuard decision, etc.).
CREATE TABLE IF NOT EXISTS execution_events (
    id            BIGSERIAL        PRIMARY KEY,
    execution_id  UUID             NOT NULL REFERENCES executions(id) ON DELETE CASCADE,
    event_type    TEXT             NOT NULL,
    payload       JSONB            NOT NULL DEFAULT '{}'::jsonb,
    created_at    TIMESTAMPTZ      NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_execution_events_exec
    ON execution_events(execution_id, created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS execution_events;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS executions;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TYPE IF EXISTS execution_status;
-- +goose StatementEnd
