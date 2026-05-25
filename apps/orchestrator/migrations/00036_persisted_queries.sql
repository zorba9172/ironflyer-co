-- +goose Up
-- persisted_queries is the production allowlist for GraphQL operations.
-- In production mode the hardening middleware rejects any operation
-- whose sha256(query) is not present here (operator principals bypass).
-- hash is the lowercase hex sha256 of the exact query string used by
-- Apollo's APQ protocol so existing clients interoperate.
CREATE TABLE IF NOT EXISTS persisted_queries (
  hash                     TEXT PRIMARY KEY,
  query                    TEXT NOT NULL,
  operation_name           TEXT,
  registered_by_tenant_id  UUID,
  registered_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_used_at             TIMESTAMPTZ,
  use_count                BIGINT NOT NULL DEFAULT 0,
  active                   BOOLEAN NOT NULL DEFAULT true
);
CREATE INDEX IF NOT EXISTS idx_persisted_queries_active ON persisted_queries(active, last_used_at DESC);
CREATE INDEX IF NOT EXISTS idx_persisted_queries_op     ON persisted_queries(operation_name) WHERE operation_name IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS persisted_queries;
