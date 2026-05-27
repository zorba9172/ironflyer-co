-- Initial database + extension setup. The pgvector image includes pgvector
-- preinstalled; we still CREATE EXTENSION here so the standard ironflyer DB
-- has it ready before migrations run.
--
-- The main `ironflyer` database is created by POSTGRES_DB env. This script
-- adds the secondary databases that Temporal + GlitchTip + WAL-G need.

\connect ironflyer

CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

\connect postgres

-- Temporal needs two databases. The auto-setup image will create the
-- schema inside them on first boot.
CREATE DATABASE temporal;
CREATE DATABASE temporal_visibility;

-- GlitchTip dedicated database.
CREATE DATABASE glitchtip;

-- WAL-G writes to MinIO (S3-compat), no separate role needed —
-- the replication role is created here so future read-replicas can
-- connect to streaming replication.
CREATE ROLE replicator WITH REPLICATION LOGIN PASSWORD 'replicator-placeholder';
