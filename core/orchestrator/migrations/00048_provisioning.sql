-- +goose Up
-- ProvisioningVault — Ironflyer-as-issuer revenue rails (Stripe Connect,
-- domain reseller, transactional-email partner, hosting). Every
-- transaction or usage event on a downstream rail Ironflyer issued for
-- a tenant project earns a configurable Ironflyer cut, recorded here
-- as an append-only ledger of forever revenue lines.
--
-- See core/orchestrator/internal/business/provisioning/ for the Go
-- contract; this migration is the persistence half.
--
-- Schema split:
--   provisioned_resources   one row per rail Ironflyer provisioned for
--                           a (tenant, project) pair; the lifecycle
--                           record (pending → active → suspended → closed).
--   revenue_policies        per-kind SharePct + MinFee + cadence so
--                           operators can retune the cut without a
--                           redeploy. Seeded from DefaultPolicies().
--   revenue_events          append-only RevenueEvent rows. Idempotent
--                           against (resource_id, external_ref) so a
--                           Stripe Connect application_fee.created
--                           webhook redelivery folds onto the same row.
--   provisioning_operations op_key dedupe log for connector-driven
--                           mutations (Provision, RecordRevenue,
--                           Suspend) — mirrors wallet_operations.

CREATE TABLE IF NOT EXISTS provisioned_resources (
  id          UUID PRIMARY KEY,
  tenant_id   UUID NOT NULL,
  project_id  UUID NOT NULL,
  kind        TEXT NOT NULL CHECK (kind IN (
                'stripe-connect','cloudflare-domain','resend-email','hosting'
              )),
  external_id TEXT,
  status      TEXT NOT NULL CHECK (status IN (
                'pending','active','suspended','closed'
              )),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- (tenant_id, external_id) is the natural dedupe key — a re-onboarding
-- of the same Stripe Connect account must fold onto the existing row.
-- The partial unique index lets pending rows with NULL external_id
-- (rare; only when the connector failed mid-onboarding) coexist.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_resources_tenant_external
  ON provisioned_resources(tenant_id, external_id)
  WHERE external_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_resources_tenant_project
  ON provisioned_resources(tenant_id, project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_resources_kind_status
  ON provisioned_resources(kind, status);

CREATE TABLE IF NOT EXISTS revenue_policies (
  kind            TEXT PRIMARY KEY CHECK (kind IN (
                    'stripe-connect','cloudflare-domain','resend-email','hosting'
                  )),
  share_pct       NUMERIC(8,6) NOT NULL CHECK (share_pct >= 0 AND share_pct <= 1),
  min_fee_usd     NUMERIC(18,6) NOT NULL DEFAULT 0,
  billing_cadence TEXT NOT NULL CHECK (billing_cadence IN (
                    'per-transaction','monthly-aggregate'
                  )),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Seed conservative defaults so a fresh boot has working policy rows
-- without a manual INSERT. Operators tune via UPDATE — the Go
-- PolicyStore reads on every cut computation so changes land live.
INSERT INTO revenue_policies(kind, share_pct, min_fee_usd, billing_cadence)
VALUES
  ('stripe-connect',    0.015, 0.05, 'per-transaction'),
  ('cloudflare-domain', 0.200, 1.00, 'monthly-aggregate'),
  ('resend-email',      0.100, 0.50, 'monthly-aggregate'),
  ('hosting',           0.150, 1.00, 'monthly-aggregate')
ON CONFLICT (kind) DO NOTHING;

CREATE TABLE IF NOT EXISTS revenue_events (
  id                  UUID PRIMARY KEY,
  resource_id         UUID NOT NULL REFERENCES provisioned_resources(id) ON DELETE CASCADE,
  occurred_at         TIMESTAMPTZ NOT NULL,
  gross_amount_usd    NUMERIC(18,6) NOT NULL CHECK (gross_amount_usd > 0),
  ironflyer_cut_usd   NUMERIC(18,6) NOT NULL CHECK (ironflyer_cut_usd >= 0),
  external_ref        TEXT,
  ledger_entry_id     TEXT
);
-- Idempotency: (resource_id, external_ref) is the natural dedupe key
-- for webhook redeliveries. Partial index so historical rows without
-- an external ref (manually backfilled, say) remain valid.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_revenue_resource_ref
  ON revenue_events(resource_id, external_ref)
  WHERE external_ref IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_revenue_resource_time
  ON revenue_events(resource_id, occurred_at DESC);

CREATE TABLE IF NOT EXISTS provisioning_operations (
  op_key       TEXT PRIMARY KEY,
  op_type      TEXT NOT NULL CHECK (op_type IN (
                 'provision','record_revenue','suspend'
               )),
  external_ref TEXT,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS provisioning_operations;
DROP INDEX IF EXISTS idx_revenue_resource_time;
DROP INDEX IF EXISTS uniq_revenue_resource_ref;
DROP TABLE IF EXISTS revenue_events;
DROP TABLE IF EXISTS revenue_policies;
DROP INDEX IF EXISTS idx_resources_kind_status;
DROP INDEX IF EXISTS idx_resources_tenant_project;
DROP INDEX IF EXISTS uniq_resources_tenant_external;
DROP TABLE IF EXISTS provisioned_resources;
