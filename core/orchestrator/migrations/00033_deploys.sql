-- +goose Up
-- V22 Deploy plane (Wave 2 / Trust). The deploys table is the durable
-- source of truth for every provider-side deployment Ironflyer drives
-- (Vercel v1 ships first; Fly / Cloudflare / k8s land later under the
-- same Adapter contract).
--
-- A row moves through the deploy_status enum below in lock-step with
-- the deploy.Service state machine: planned → preview_building →
-- preview_ready → awaiting_approval → promoting → promoted, with
-- rolled_back / failed / cancelled as the three terminal off-ramps.
-- +goose StatementBegin
DO $$ BEGIN
  CREATE TYPE deploy_status AS ENUM (
    'planned', 'preview_building', 'preview_ready',
    'awaiting_approval', 'promoting', 'promoted',
    'rolled_back', 'failed', 'cancelled'
  );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;
-- +goose StatementEnd

CREATE TABLE IF NOT EXISTS deploys (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  project_id UUID NOT NULL,
  execution_id UUID,
  blueprint_id TEXT,
  target TEXT NOT NULL,                  -- 'vercel' | 'fly' | 'cloudflare' | 'k8s'
  environment TEXT NOT NULL CHECK (environment IN ('preview','production')),
  status deploy_status NOT NULL DEFAULT 'planned',
  provider_deployment_id TEXT,
  preview_url TEXT,
  production_url TEXT,
  diff_hash TEXT,
  artifact_hash TEXT,
  gate_summary JSONB NOT NULL DEFAULT '{}',
  cost_usd NUMERIC(18,6) NOT NULL DEFAULT 0,
  metadata JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  preview_ready_at TIMESTAMPTZ,
  promoted_at TIMESTAMPTZ,
  rolled_back_at TIMESTAMPTZ,
  CHECK (cost_usd >= 0)
);
CREATE INDEX IF NOT EXISTS idx_deploys_tenant ON deploys(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_deploys_status ON deploys(status, created_at DESC);

-- deploy_events is a thin per-deploy event ledger that mirrors the
-- execution_events pattern: every state transition the Service drives
-- writes one row here so the GraphQL deployFeed subscription has
-- something to replay.
CREATE TABLE IF NOT EXISTS deploy_events (
  id BIGSERIAL PRIMARY KEY,
  deploy_id UUID NOT NULL REFERENCES deploys(id) ON DELETE CASCADE,
  event_type TEXT NOT NULL,
  payload JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_deploy_events_deploy ON deploy_events(deploy_id, id);

-- +goose Down
DROP TABLE IF EXISTS deploy_events;
DROP TABLE IF EXISTS deploys;
DROP TYPE IF EXISTS deploy_status;
