-- +goose Up
-- abuse_scores stores the current per-(tenant,user) abuse posture. The
-- tier column is derived from the score by the engine on write so policy
-- consumers can read tier without recomputing thresholds.
--
-- Score range is enforced 0..100 at the DB layer; the engine clamps and
-- the migration locks the contract so any direct write that violates
-- the band is rejected. The signals JSONB column carries the last
-- recorded per-type weight aggregation so the dashboard can render the
-- breakdown without re-querying abuse_signals.
CREATE TABLE IF NOT EXISTS abuse_scores (
  tenant_id UUID NOT NULL,
  user_id   UUID,
  score     INT NOT NULL CHECK (score BETWEEN 0 AND 100),
  tier      TEXT NOT NULL CHECK (tier IN ('normal','elevated','restricted','blocked')),
  signals   JSONB NOT NULL DEFAULT '{}'::jsonb,
  reason    TEXT,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_abuse_scores_tier ON abuse_scores(tier, updated_at DESC);

-- abuse_signals is the append-only log of raw signal events. The engine
-- recomputes Score = SUM(weight) over the last 24h window from this
-- table so callers can replay scoring or backfill thresholds without a
-- destructive update. weight is signed: positive raises risk, negative
-- counts as a recovery signal (e.g. successful MFA challenge).
CREATE TABLE IF NOT EXISTS abuse_signals (
  id           BIGSERIAL PRIMARY KEY,
  tenant_id    UUID NOT NULL,
  user_id      UUID,
  signal_type  TEXT NOT NULL,
  weight       INT NOT NULL,
  context      JSONB NOT NULL DEFAULT '{}'::jsonb,
  recorded_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_abuse_signals_tenant ON abuse_signals(tenant_id, recorded_at DESC);
CREATE INDEX IF NOT EXISTS idx_abuse_signals_user   ON abuse_signals(tenant_id, user_id, recorded_at DESC);

-- +goose Down
DROP TABLE IF EXISTS abuse_signals;
DROP TABLE IF EXISTS abuse_scores;
