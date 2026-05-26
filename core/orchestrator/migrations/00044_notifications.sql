-- +goose Up
CREATE TABLE IF NOT EXISTS notifications (
  id           UUID PRIMARY KEY,
  user_id      UUID NOT NULL,
  kind         TEXT NOT NULL,
  title        TEXT NOT NULL,
  body         TEXT NOT NULL,
  link         TEXT,
  severity     TEXT NOT NULL CHECK (severity IN ('info','warning','critical')),
  read_at      TIMESTAMPTZ,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_notifications_user_unread
  ON notifications (user_id, created_at DESC)
  WHERE read_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_notifications_user_all
  ON notifications (user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS notification_outbox (
  id               UUID PRIMARY KEY,
  user_id          UUID NOT NULL,
  kind             TEXT NOT NULL,
  payload          JSONB NOT NULL,
  email_target     BOOLEAN NOT NULL DEFAULT false,
  inapp_target     BOOLEAN NOT NULL DEFAULT false,
  attempts         INT NOT NULL DEFAULT 0,
  next_attempt_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_error       TEXT,
  delivered_at     TIMESTAMPTZ,
  dead_lettered_at TIMESTAMPTZ,
  email_sent_at    TIMESTAMPTZ,
  inapp_sent_at    TIMESTAMPTZ,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_outbox_due
  ON notification_outbox (next_attempt_at)
  WHERE delivered_at IS NULL AND dead_lettered_at IS NULL;

CREATE TABLE IF NOT EXISTS notification_idempotency (
  key         TEXT PRIMARY KEY,
  outbox_id   UUID NOT NULL REFERENCES notification_outbox(id) ON DELETE CASCADE,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS notification_idempotency;
DROP TABLE IF EXISTS notification_outbox;
DROP TABLE IF EXISTS notifications;
