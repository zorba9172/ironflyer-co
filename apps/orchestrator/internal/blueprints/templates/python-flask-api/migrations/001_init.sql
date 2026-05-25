-- Initial schema for the Ironflyer Flask API blueprint.
-- Apply with: psql "$DATABASE_URL" -f migrations/001_init.sql
-- (SQLAlchemy's db.create_all() will create the same tables on app
-- start; this file is for environments where migrations are managed
-- out-of-band.)

CREATE TABLE IF NOT EXISTS users (
    id         SERIAL PRIMARY KEY,
    email      VARCHAR(255) NOT NULL UNIQUE,
    name       VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS items (
    id         SERIAL PRIMARY KEY,
    title      VARCHAR(255) NOT NULL,
    body       TEXT,
    owner_id   INTEGER REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS items_owner_created_idx
    ON items (owner_id, created_at DESC);
