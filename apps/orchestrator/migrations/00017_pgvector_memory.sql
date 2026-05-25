-- Memory records backed by Postgres + pgvector.
--
-- This migration installs the `vector` extension and the `memory_records`
-- table so operators who already run Aurora Postgres (Round 11 Pulumi
-- provisioned it) can host the four-dimensional memory layer in the same
-- database as the budget ledger, leads, federation, etc. — without
-- standing up SurrealDB just for memory.
--
-- Embedding dimension note:
--   The `embedding` column is `vector(1024)` because the default
--   embedder is `BAAI/bge-m3` (1024-dim). Operators who pin
--   `HF_EMBEDDINGS_MODEL` to a model with a different output dimension
--   MUST alter the column type before turning on `MEMORY_BACKEND=pgvector`
--   — pgvector enforces the declared dim at INSERT time. The pgvector
--   store accepts NULL embeddings (smart-fallback path) so the column
--   stays nullable on purpose.
--
-- HNSW with cosine distance is the production-grade index for semantic
-- search; `m=16, ef_construction=64` is the upstream-recommended starting
-- point that balances build time vs. recall on millions of rows.

-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS vector;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS memory_records (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL,
    project_id  TEXT,
    kind        TEXT NOT NULL,
    story_id    TEXT,
    gate_name   TEXT,
    title       TEXT,
    body        TEXT NOT NULL,
    tags        JSONB,
    confidence  DOUBLE PRECISION NOT NULL DEFAULT 0,
    embedding   vector(1024),
    metadata    JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_memory_user ON memory_records (user_id, project_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_memory_kind ON memory_records (kind);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_memory_created_at ON memory_records (created_at DESC);
-- +goose StatementEnd

-- HNSW index over cosine distance — the production-grade choice for
-- semantic ranking. Skips when pgvector lacks HNSW support (very old
-- pgvector versions); operators on those should upgrade pgvector >= 0.5.
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_memory_embedding_hnsw
  ON memory_records USING hnsw (embedding vector_cosine_ops)
  WITH (m = 16, ef_construction = 64);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS memory_records;
-- Note: we don't drop the extension because other tables may use it.
-- +goose StatementEnd
