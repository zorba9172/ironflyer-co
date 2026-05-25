-- +goose Up
-- +goose StatementBegin
-- Stateful session registry. JWTs stay primarily stateless (signature +
-- exp), but every issued token also lands here keyed by its `jti` claim
-- so we can:
--   - list "my sessions" with ip_address / user_agent / last_seen_at
--   - revoke a single session OR everything-but-current
--   - reject tokens whose row has revoked_at IS NOT NULL
-- The middleware checks the row via a 60s Redis-backed cache so the DB
-- isn't hit on every request.
CREATE TABLE IF NOT EXISTS sessions (
    jti          TEXT        PRIMARY KEY,
    user_id      UUID        NOT NULL,
    issued_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at   TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ip_address   TEXT        NOT NULL DEFAULT '',
    user_agent   TEXT        NOT NULL DEFAULT '',
    revoked_at   TIMESTAMPTZ
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_sessions_user_active
    ON sessions(user_id) WHERE revoked_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS sessions;
-- +goose StatementEnd
