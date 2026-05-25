-- +goose Up
-- +goose StatementBegin
-- Email verification for signup + email change. Verified-or-not, a user
-- can sign in; gated features (paid plans, deploys, custom domains)
-- require email_verified_at IS NOT NULL via requireVerifiedEmail.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS email_verified_at TIMESTAMPTZ;
-- +goose StatementEnd

-- +goose StatementBegin
-- email_verifications carries both signup and email-change tokens.
-- `kind` discriminates which flow the token belongs to:
--   - 'signup' : initial post-signup verification
--   - 'change' : verifies the NEW email during an email-change flow
-- The `new_email` column is only populated for change-flow rows; the
-- confirm step uses it to flip the user's email to the proven address.
CREATE TABLE IF NOT EXISTS email_verifications (
    token_hash TEXT        PRIMARY KEY,
    user_id    UUID        NOT NULL,
    kind       TEXT        NOT NULL DEFAULT 'signup',
    new_email  TEXT        NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    used_at    TIMESTAMPTZ
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_email_verifications_user
    ON email_verifications(user_id, kind);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS email_verifications;
ALTER TABLE users DROP COLUMN IF EXISTS email_verified_at;
-- +goose StatementEnd
