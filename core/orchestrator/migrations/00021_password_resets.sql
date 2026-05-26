-- +goose Up
-- +goose StatementBegin
-- Password-reset tokens. The plaintext is emailed; only the SHA-256
-- hash is stored so a compromised DB cannot trivially mint resets.
-- TTL is enforced at validation time (default 1h, see password_reset.go).
CREATE TABLE IF NOT EXISTS password_resets (
    token_hash TEXT        PRIMARY KEY,
    user_id    UUID        NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_password_resets_user
    ON password_resets(user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS password_resets;
-- +goose StatementEnd
