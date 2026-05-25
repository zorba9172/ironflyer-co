-- +goose Up
CREATE TABLE IF NOT EXISTS secret_refs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  project_id UUID,
  name TEXT NOT NULL,                       -- e.g. "STRIPE_SECRET_KEY"
  backend TEXT NOT NULL,                    -- 'env' | 'aws_secrets' | 'gcp_secrets' | 'vault' | 'kv'
  backend_ref TEXT NOT NULL,                -- backend-specific reference (path / arn / key)
  release_class TEXT NOT NULL CHECK (release_class IN ('build_time_reference','runtime_mount','operator_break_glass')),
  version INT NOT NULL DEFAULT 1,
  rotated_at TIMESTAMPTZ,
  last_released_at TIMESTAMPTZ,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(tenant_id, project_id, name)
);
CREATE INDEX IF NOT EXISTS idx_secret_refs_tenant ON secret_refs(tenant_id, created_at DESC);

CREATE TABLE IF NOT EXISTS secret_releases (
  id BIGSERIAL PRIMARY KEY,
  secret_ref_id UUID NOT NULL REFERENCES secret_refs(id) ON DELETE CASCADE,
  tenant_id UUID NOT NULL,
  execution_id UUID,
  workspace_id TEXT,
  policy_decision_id TEXT NOT NULL,
  released_to TEXT NOT NULL,                -- 'workspace_mount' | 'deploy_provider' | 'operator_session'
  released_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ NOT NULL,
  redaction_proof TEXT NOT NULL DEFAULT 'sha256:redacted'
);
CREATE INDEX IF NOT EXISTS idx_secret_releases_secret ON secret_releases(secret_ref_id, released_at DESC);

-- +goose Down
DROP TABLE IF EXISTS secret_releases;
DROP TABLE IF EXISTS secret_refs;
