-- +goose Up
-- +goose StatementBegin
--
-- V22 blueprints (Agent 6) — the data-driven starter registry that
-- replaces the ~30 hand-written scaffolders the proof pack retired.
-- Two tables:
--
--   blueprint_stats  — one row per blueprint id, the rolled-up
--                      counters/sums the Blueprint Profit Dashboard
--                      ranks from. RecordRun does an atomic UPSERT
--                      against this row so dashboard reads stay O(1)
--                      without scanning the per-run history.
--
--   blueprint_runs   — one row per execution that used a blueprint,
--                      kept so we can compute time-windowed cohorts,
--                      audit refunds, and re-derive stats if the
--                      rollup is ever corrupted.
--
-- IDs are text because the registry is built in code (NewBuiltInRegistry)
-- with stable string keys ("nextjs-production", "go-http-api", "static-landing");
-- a UUID FK would force an additional bootstrap path with no value.
-- tenant_id / execution_id are plain UUIDs (same convention as
-- 00025_ledger.sql) — the executions table is owned by Agent 4 and we
-- don't want a hard FK ordering between parallel agents.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS blueprint_stats (
  blueprint_id              TEXT          PRIMARY KEY,
  executions                BIGINT        NOT NULL DEFAULT 0,
  preview_success           BIGINT        NOT NULL DEFAULT 0,
  refunds                   BIGINT        NOT NULL DEFAULT 0,
  total_revenue_usd         NUMERIC(18,6) NOT NULL DEFAULT 0,
  total_cost_usd            NUMERIC(18,6) NOT NULL DEFAULT 0,
  total_completion_score    NUMERIC(18,6) NOT NULL DEFAULT 0,
  repair_count              BIGINT        NOT NULL DEFAULT 0,
  time_to_preview_seconds_sum BIGINT      NOT NULL DEFAULT 0,
  time_to_preview_count     BIGINT        NOT NULL DEFAULT 0,
  updated_at                TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS blueprint_runs (
  id                       UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
  blueprint_id             TEXT          NOT NULL,
  execution_id             UUID          NOT NULL,
  tenant_id                UUID          NOT NULL,
  revenue_usd              NUMERIC(18,6) NOT NULL DEFAULT 0,
  cost_usd                 NUMERIC(18,6) NOT NULL DEFAULT 0,
  completion_score         NUMERIC(5,4)  NOT NULL DEFAULT 0,
  preview_success          BOOLEAN       NOT NULL DEFAULT false,
  repaired                 BOOLEAN       NOT NULL DEFAULT false,
  time_to_preview_seconds  INT,
  refunded                 BOOLEAN       NOT NULL DEFAULT false,
  created_at               TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_bruns_blueprint
    ON blueprint_runs(blueprint_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_bruns_execution
    ON blueprint_runs(execution_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS blueprint_runs;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS blueprint_stats;
-- +goose StatementEnd
