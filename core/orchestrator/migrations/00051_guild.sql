-- +goose Up
-- +goose StatementBegin
--
-- V22 FinisherGuild — two-sided monetization marketplace.
--
-- NOTE: number 00099 is a placeholder while parallel agents land their
-- own migrations (provisioning + compliance currently collide at
-- 00048). The wireup author MUST renumber this to the next free slot
-- after their migrations land (likely 0005x) before merging — goose
-- requires monotonic ordering and silently ignores filename gaps but
-- collides on duplicates.
--
-- Tables:
--   finisher_profiles  — one row per enrolled finisher (UserID unique).
--   guild_tasks        — open work units the requestor crowd-sources.
--   guild_bids         — finisher offers on a task. price <= task floor.
--   templates          — community-authored starter kits (slug unique).
--   template_installs  — per-install rev-share ledger.
--   guild_payouts      — queued cash-outs to finishers (Stripe Connect
--                        transfer wiring is a TODO for the provisioning
--                        agent — rows land in 'pending').
--   guild_operations   — opKey-keyed dedupe for accept_bid /
--                        install_template under Temporal retries.
--
-- All money is NUMERIC(18,6) USD — matches wallets / ledger_entries.
-- IDs are TEXT (not UUID) on tables whose ids may be generated client-
-- side (finisher_profiles.user_id is FK-ish to users.id which is UUID,
-- but stored as TEXT to avoid a hard cross-schema constraint).

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS finisher_profiles (
  id                    TEXT          PRIMARY KEY,
  user_id               TEXT          NOT NULL UNIQUE,
  display_name          TEXT          NOT NULL,
  skills                TEXT[]        NOT NULL DEFAULT '{}',
  hourly_rate_usd       NUMERIC(18,6) NOT NULL DEFAULT 0 CHECK (hourly_rate_usd >= 0),
  completed_task_count  INT           NOT NULL DEFAULT 0,
  rating                NUMERIC(3,2)  NOT NULL DEFAULT 0 CHECK (rating >= 0 AND rating <= 5),
  verified              BOOLEAN       NOT NULL DEFAULT FALSE,
  created_at            TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS guild_tasks (
  id              TEXT          PRIMARY KEY,
  project_id      TEXT          NOT NULL,
  tenant_id       TEXT          NOT NULL,
  gate_failure_id TEXT,
  title           TEXT          NOT NULL,
  description     TEXT          NOT NULL DEFAULT '',
  price_usd_floor NUMERIC(18,6) NOT NULL CHECK (price_usd_floor > 0),
  sla_hours       INT           NOT NULL DEFAULT 0,
  status          TEXT          NOT NULL
    CHECK (status IN ('open','bidding','in-progress','review','accepted','rejected','expired')),
  assigned_to     TEXT,
  created_at      TIMESTAMPTZ   NOT NULL DEFAULT now(),
  accepted_at     TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_guild_tasks_tenant   ON guild_tasks(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_guild_tasks_project  ON guild_tasks(project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_guild_tasks_status   ON guild_tasks(status, created_at DESC);

CREATE TABLE IF NOT EXISTS guild_bids (
  id              TEXT          PRIMARY KEY,
  task_id         TEXT          NOT NULL REFERENCES guild_tasks(id) ON DELETE CASCADE,
  finisher_id     TEXT          NOT NULL REFERENCES finisher_profiles(id) ON DELETE CASCADE,
  price_usd       NUMERIC(18,6) NOT NULL CHECK (price_usd > 0),
  estimated_hours INT           NOT NULL DEFAULT 0,
  note            TEXT          NOT NULL DEFAULT '',
  status          TEXT          NOT NULL
    CHECK (status IN ('open','won','lost','withdrawn')),
  created_at      TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_guild_bids_task     ON guild_bids(task_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_guild_bids_finisher ON guild_bids(finisher_id, created_at DESC);

CREATE TABLE IF NOT EXISTS templates (
  id              TEXT          PRIMARY KEY,
  author_user_id  TEXT          NOT NULL,
  slug            TEXT          NOT NULL UNIQUE,
  name            TEXT          NOT NULL,
  description     TEXT          NOT NULL DEFAULT '',
  price_usd       NUMERIC(18,6) NOT NULL CHECK (price_usd >= 0),
  gates_passed    TEXT[]        NOT NULL DEFAULT '{}',
  install_count   INT           NOT NULL DEFAULT 0,
  verified        BOOLEAN       NOT NULL DEFAULT FALSE,
  created_at      TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_templates_author ON templates(author_user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS template_installs (
  id                TEXT          PRIMARY KEY,
  template_id       TEXT          NOT NULL REFERENCES templates(id) ON DELETE CASCADE,
  project_id        TEXT          NOT NULL,
  tenant_id         TEXT          NOT NULL,
  amount_usd        NUMERIC(18,6) NOT NULL CHECK (amount_usd >= 0),
  author_payout_usd NUMERIC(18,6) NOT NULL CHECK (author_payout_usd >= 0),
  platform_cut_usd  NUMERIC(18,6) NOT NULL CHECK (platform_cut_usd >= 0),
  installed_at      TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_template_installs_template ON template_installs(template_id, installed_at DESC);
CREATE INDEX IF NOT EXISTS idx_template_installs_tenant   ON template_installs(tenant_id, installed_at DESC);

CREATE TABLE IF NOT EXISTS guild_payouts (
  id                TEXT          PRIMARY KEY,
  task_id           TEXT          NOT NULL REFERENCES guild_tasks(id) ON DELETE CASCADE,
  finisher_id       TEXT          NOT NULL REFERENCES finisher_profiles(id) ON DELETE CASCADE,
  amount_usd        NUMERIC(18,6) NOT NULL CHECK (amount_usd >= 0),
  finisher_cut_usd  NUMERIC(18,6) NOT NULL CHECK (finisher_cut_usd >= 0),
  platform_cut_usd  NUMERIC(18,6) NOT NULL CHECK (platform_cut_usd >= 0),
  status            TEXT          NOT NULL CHECK (status IN ('pending','paid','failed')),
  created_at        TIMESTAMPTZ   NOT NULL DEFAULT now(),
  completed_at      TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_guild_payouts_finisher ON guild_payouts(finisher_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_guild_payouts_status   ON guild_payouts(status, created_at DESC);

CREATE TABLE IF NOT EXISTS guild_operations (
  op_key      TEXT          PRIMARY KEY,
  op_type     TEXT          NOT NULL
    CHECK (op_type IN ('accept_bid','install_template','reject_task','expire_task')),
  amount_usd  NUMERIC(18,6) NOT NULL DEFAULT 0,
  status      TEXT          NOT NULL CHECK (status IN ('succeeded','failed')),
  error_code  TEXT,
  created_at  TIMESTAMPTZ   NOT NULL DEFAULT now()
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS guild_operations;
DROP TABLE IF EXISTS guild_payouts;
DROP TABLE IF EXISTS template_installs;
DROP TABLE IF EXISTS templates;
DROP TABLE IF EXISTS guild_bids;
DROP TABLE IF EXISTS guild_tasks;
DROP TABLE IF EXISTS finisher_profiles;
-- +goose StatementEnd
