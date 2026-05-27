#!/bin/sh
# Restic hourly backup of the /data tree to Hetzner Storage Box.
#
# Strategy:
#   - Postgres data dir is excluded (covered by WAL-G to MinIO + an
#     additional cross-host snapshot below).
#   - Loki + VictoriaMetrics are excluded (telemetry is rebuildable).
#   - Everything else (Surreal, ClickHouse, Redpanda, MinIO, Caddy
#     state, Redis AOF) gets a deduped snapshot.
#   - Retention: 24 hourly + 14 daily + 8 weekly + 12 monthly.
set -eu

LOG_PREFIX="$(date -u +%FT%TZ) restic"

echo "${LOG_PREFIX} starting hourly snapshot"

restic backup /data \
  --exclude=/data/postgres \
  --exclude=/data/loki \
  --exclude=/data/victoriametrics \
  --exclude=/data/vmagent \
  --exclude=/data/caddy/data/caddy/locks \
  --tag hourly \
  --host ironflyer-prod

echo "${LOG_PREFIX} pruning old snapshots"

restic forget \
  --keep-hourly 24 \
  --keep-daily 14 \
  --keep-weekly 8 \
  --keep-monthly 12 \
  --prune

echo "${LOG_PREFIX} done"
