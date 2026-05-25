-- +goose Up
-- +goose StatementBegin
-- Per-user telemetry preference. The instance-wide
-- IRONFLYER_TELEMETRY_OPT_OUT env still trumps this; the column matters
-- only when the env flag is unset and a user has explicitly opted out
-- from their privacy settings.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS telemetry_opt_out BOOLEAN NOT NULL DEFAULT FALSE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users DROP COLUMN IF EXISTS telemetry_opt_out;
-- +goose StatementEnd
