-- +goose Up
-- Domains attached to customer deployments. This is intentionally separate
-- from deploys: a project can have multiple domains, domains can outlive a
-- single deploy, and registrar purchase state is a different lifecycle from
-- build/promote/rollback.
CREATE TABLE IF NOT EXISTS deploy_domains (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  project_id UUID NOT NULL,
  deploy_id UUID REFERENCES deploys(id) ON DELETE SET NULL,
  hostname TEXT NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN ('managed_subdomain','connected_domain','registered_domain')),
  status TEXT NOT NULL CHECK (status IN ('pending_dns','verifying','live','failed','removed')),
  provider TEXT NOT NULL DEFAULT 'ironflyer',
  registrar TEXT,
  is_primary BOOLEAN NOT NULL DEFAULT false,
  dns_records JSONB NOT NULL DEFAULT '[]',
  verification_status TEXT NOT NULL DEFAULT 'dns_pending',
  certificate_status TEXT NOT NULL DEFAULT 'pending' CHECK (certificate_status IN ('pending','active','failed')),
  instructions TEXT NOT NULL DEFAULT '',
  metadata JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  verified_at TIMESTAMPTZ,
  live_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS deploy_domains_tenant_hostname_live
  ON deploy_domains(tenant_id, lower(hostname))
  WHERE status <> 'removed';

CREATE INDEX IF NOT EXISTS idx_deploy_domains_project
  ON deploy_domains(tenant_id, project_id, is_primary DESC, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_deploy_domains_deploy
  ON deploy_domains(deploy_id);

-- +goose Down
DROP TABLE IF EXISTS deploy_domains;
