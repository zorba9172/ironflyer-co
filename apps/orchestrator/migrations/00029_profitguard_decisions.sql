-- +goose Up
-- +goose StatementBegin
--
-- V22 ProfitGuard (Agent 5) — append-only audit of every economic
-- enforcement decision Profit Guard makes at the runtime gates listed
-- in
--   docs/ironflyer_deep_atomic_plan_v22_profit_scale_proof_pack/
--     01-unit-economics/03-profit-guard-policy.md
--
-- One row per Decide() call so the profit dashboard and the
-- per-execution drill-down can reconstruct exactly which enforcement
-- point triggered which action and why. Append-only by contract:
-- there is no UPDATE / DELETE surface in internal/profitguard/.
--
-- execution_id is a plain UUID (no FK) to stay decoupled from the
-- executions table created by Agent 4 — the integration agent wires
-- the join at query time.

CREATE TABLE IF NOT EXISTS profit_guard_decisions (
  id BIGSERIAL PRIMARY KEY,
  execution_id UUID NOT NULL,
  enforcement_point TEXT NOT NULL,
  decision TEXT NOT NULL,
  reason TEXT NOT NULL,
  spent_usd NUMERIC(18,6) NOT NULL,
  reserved_usd NUMERIC(18,6) NOT NULL,
  estimated_step_cost_usd NUMERIC(18,6) NOT NULL,
  expected_completion_delta NUMERIC(5,4) NOT NULL,
  expected_margin_pct NUMERIC(7,4),
  risk_score NUMERIC(5,4),
  recommended_provider TEXT,
  metadata JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_pgd_execution ON profit_guard_decisions(execution_id, created_at);
CREATE INDEX IF NOT EXISTS idx_pgd_decision ON profit_guard_decisions(decision, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS profit_guard_decisions;
-- +goose StatementEnd
