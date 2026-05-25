-- +goose Up
-- V22 Deploy Approvals. A production promote MUST be backed by a
-- decided row in deploy_approvals (or an explicit auto-deploy policy
-- obligation) — see ARCHITECTURE_POLICY_SECURITY.md "Deploy Approvals"
-- and the deploy.Service.Promote enforcement path.
--
-- requested_by_user_id may be NULL when an AI execution opened the
-- approval; decided_by_user_id is set when a human (or scripted policy
-- automation) flips the row to approved / rejected.
CREATE TABLE IF NOT EXISTS deploy_approvals (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  deploy_id UUID NOT NULL REFERENCES deploys(id) ON DELETE CASCADE,
  tenant_id UUID NOT NULL,
  requested_by_user_id UUID,             -- AI executions have NULL
  decided_by_user_id UUID,
  status TEXT NOT NULL CHECK (status IN ('pending','approved','rejected','expired','withdrawn')),
  diff_hash TEXT NOT NULL,
  artifact_hash TEXT NOT NULL,
  gate_summary JSONB NOT NULL,
  cost_impact_usd NUMERIC(18,6) NOT NULL DEFAULT 0,
  expires_at TIMESTAMPTZ NOT NULL,
  decision_note TEXT,
  policy_decision_id TEXT,
  audit_chain_event_id TEXT,
  requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  decided_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_deploy_approvals_pending
  ON deploy_approvals(tenant_id, status, requested_at DESC)
  WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_deploy_approvals_deploy
  ON deploy_approvals(deploy_id, requested_at DESC);

-- +goose Down
DROP TABLE IF EXISTS deploy_approvals;
