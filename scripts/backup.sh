#!/usr/bin/env bash
# Ironflyer Postgres backup.
#
# Performs a logical backup with `pg_dump --format=custom --compress=9`
# against ${POSTGRES_URL}, then uploads to ${BACKUP_S3_URI}/<timestamp>.dump.
# Stale objects beyond ${BACKUP_RETENTION_DAYS:-30} are pruned from the prefix.
#
# Primary uploader: AWS CLI (`aws s3 cp`, `aws s3 ls`, `aws s3 rm`).
# Fallback: if `aws` is unavailable, `rclone` can be substituted by setting
#   IRONFLYER_BACKUP_TOOL=rclone and ensuring the remote in ${BACKUP_S3_URI}
#   is configured under `rclone config` (e.g. "myremote:bucket/prefix").
#   rclone equivalents:
#     upload : rclone copy "$LOCAL" "$BACKUP_S3_URI"
#     list   : rclone lsl  "$BACKUP_S3_URI"
#     delete : rclone delete --min-age "${BACKUP_RETENTION_DAYS}d" "$BACKUP_S3_URI"
#
# Designed to be invoked by cron or a Kubernetes CronJob — fails loud on
# any error so the orchestrator sees a non-zero exit.
#
# Required env:
#   POSTGRES_URL              postgres://user:pass@host:5432/db?sslmode=...
#   BACKUP_S3_URI             s3://bucket/prefix  (no trailing slash required)
# Optional env:
#   BACKUP_RETENTION_DAYS     default 30
#   AWS_REGION / AWS_PROFILE  forwarded to the aws CLI
#   IRONFLYER_BACKUP_TOOL     "aws" (default) | "rclone"
#   BACKUP_S3_BACKEND         "aws" (default) | "spaces" | "r2" | "minio".
#                             When non-default, exports AWS_ENDPOINT_URL_S3
#                             so AWS CLI v2 transparently routes to the
#                             S3-compatible endpoint:
#                               spaces : https://${DO_SPACES_REGION}.digitaloceanspaces.com
#                                        (DO_SPACES_REGION defaults to nyc3)
#                               r2     : https://${R2_ACCOUNT_ID}.r2.cloudflarestorage.com
#                               minio  : ${MINIO_ENDPOINT}
#                             Operators should also set the matching access
#                             credentials as AWS_ACCESS_KEY_ID /
#                             AWS_SECRET_ACCESS_KEY (Spaces / R2 / MinIO all
#                             accept the same S3 sig-v4 headers).

set -euo pipefail

log() { printf '[backup] %s %s\n' "$(date -u +%FT%TZ)" "$*"; }
die() { printf '[backup] ERROR %s %s\n' "$(date -u +%FT%TZ)" "$*" >&2; exit 1; }

# Resolve AWS_ENDPOINT_URL_S3 from BACKUP_S3_BACKEND so the rest of the
# script can keep using `aws s3` verbs unchanged. AWS CLI v2 honours
# AWS_ENDPOINT_URL_S3 automatically; for v1 fallback the operator can
# pass --endpoint-url via AWS_CLI_OPTS (unused here today).
case "${BACKUP_S3_BACKEND:-aws}" in
  aws) ;;
  spaces)
    : "${DO_SPACES_REGION:=nyc3}"
    export AWS_ENDPOINT_URL_S3="https://${DO_SPACES_REGION}.digitaloceanspaces.com"
    log "BACKUP_S3_BACKEND=spaces — endpoint=${AWS_ENDPOINT_URL_S3}"
    ;;
  r2)
    : "${R2_ACCOUNT_ID:?R2_ACCOUNT_ID is required when BACKUP_S3_BACKEND=r2}"
    export AWS_ENDPOINT_URL_S3="https://${R2_ACCOUNT_ID}.r2.cloudflarestorage.com"
    log "BACKUP_S3_BACKEND=r2 — endpoint=${AWS_ENDPOINT_URL_S3}"
    ;;
  minio)
    : "${MINIO_ENDPOINT:?MINIO_ENDPOINT is required when BACKUP_S3_BACKEND=minio}"
    export AWS_ENDPOINT_URL_S3="${MINIO_ENDPOINT}"
    log "BACKUP_S3_BACKEND=minio — endpoint=${AWS_ENDPOINT_URL_S3}"
    ;;
  *)
    die "unknown BACKUP_S3_BACKEND=${BACKUP_S3_BACKEND} (expected aws|spaces|r2|minio)"
    ;;
esac

: "${POSTGRES_URL:?POSTGRES_URL is required}"
: "${BACKUP_S3_URI:?BACKUP_S3_URI is required (e.g. s3://my-bucket/ironflyer)}"

# Data-residency guard. A `us-east-1` bucket holding `eu` data violates
# GDPR; warn loudly when ${IRONFLYER_REGION} is set but doesn't appear
# anywhere in ${BACKUP_S3_URI}. We warn rather than die because operators
# legitimately name buckets without the region substring (e.g.
# `ironflyer-prod-backups` doesn't contain "ams3"). The warning
# is loud + structured so it lands in dashboards.
if [ -n "${IRONFLYER_REGION:-}" ] && [ "${IRONFLYER_REGION}" != "unknown" ]; then
  case "$BACKUP_S3_URI" in
    *"${IRONFLYER_REGION}"*)
      log "region OK: BACKUP_S3_URI contains '${IRONFLYER_REGION}'"
      ;;
    *)
      printf '[backup] WARN region check: BACKUP_S3_URI=%s does not contain region substring "%s" — verify the bucket lives in-region to keep data residency hermetic (see docs/DR_RUNBOOK.md § Backup region affinity)\n' \
        "$BACKUP_S3_URI" "$IRONFLYER_REGION" >&2
      ;;
  esac
fi

RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-30}"
TOOL="${IRONFLYER_BACKUP_TOOL:-aws}"
TS="$(date -u +%Y%m%d-%H%M%S)"
FILENAME="ironflyer-pg-${TS}.dump"
TMPDIR="$(mktemp -d)"
LOCAL="${TMPDIR}/${FILENAME}"

trap 'rm -rf "$TMPDIR"' EXIT

case "$TOOL" in
  aws)
    command -v aws >/dev/null 2>&1 || die "aws CLI not found (set IRONFLYER_BACKUP_TOOL=rclone to use rclone)"
    ;;
  rclone)
    command -v rclone >/dev/null 2>&1 || die "rclone not found"
    ;;
  *)
    die "unknown IRONFLYER_BACKUP_TOOL=$TOOL (expected aws|rclone)"
    ;;
esac

command -v pg_dump >/dev/null 2>&1 || die "pg_dump not found in PATH"

log "dumping Postgres → ${LOCAL}"
pg_dump --format=custom --compress=9 --no-owner --no-privileges \
  --file="$LOCAL" "$POSTGRES_URL"

BYTES="$(wc -c <"$LOCAL" | tr -d ' ')"
log "dump complete: ${BYTES} bytes"

DEST="${BACKUP_S3_URI%/}/${FILENAME}"
log "uploading → ${DEST}"
if [ "$TOOL" = "aws" ]; then
  aws s3 cp "$LOCAL" "$DEST" --no-progress
else
  rclone copyto "$LOCAL" "$DEST"
fi
log "upload OK"

log "applying retention: keeping ${RETENTION_DAYS}d on ${BACKUP_S3_URI}"
if [ "$TOOL" = "aws" ]; then
  CUTOFF_EPOCH="$(($(date -u +%s) - RETENTION_DAYS * 86400))"
  # `aws s3 ls` output: 2026-05-20 03:00:01    1234567 ironflyer-pg-...dump
  aws s3 ls "${BACKUP_S3_URI%/}/" | while read -r line; do
    [ -z "$line" ] && continue
    obj_date="$(echo "$line" | awk '{print $1" "$2}')"
    obj_name="$(echo "$line" | awk '{print $4}')"
    [ -z "$obj_name" ] && continue
    case "$obj_name" in
      ironflyer-pg-*.dump) ;;
      *) continue ;;
    esac
    # Convert YYYY-MM-DD HH:MM:SS to epoch; BSD vs GNU date.
    if obj_epoch="$(date -u -d "$obj_date" +%s 2>/dev/null)"; then :; else
      obj_epoch="$(date -u -j -f '%Y-%m-%d %H:%M:%S' "$obj_date" +%s 2>/dev/null || true)"
    fi
    [ -z "${obj_epoch:-}" ] && { log "skip (unparsable date): $obj_name"; continue; }
    if [ "$obj_epoch" -lt "$CUTOFF_EPOCH" ]; then
      log "prune: $obj_name"
      aws s3 rm "${BACKUP_S3_URI%/}/$obj_name"
    fi
  done
else
  rclone delete --min-age "${RETENTION_DAYS}d" "${BACKUP_S3_URI%/}/" \
    --include 'ironflyer-pg-*.dump'
fi

log "done — backup ${FILENAME} stored at ${DEST}"
