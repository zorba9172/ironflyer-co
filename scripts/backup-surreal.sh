#!/usr/bin/env bash
# Ironflyer SurrealDB backup.
#
# Runs `surreal export` against ${SURREAL_URL} and uploads to
# ${BACKUP_S3_URI}/<timestamp>.surql with the same retention semantics as
# scripts/backup.sh. SurrealDB is optional in the stack; if SURREAL_URL is
# empty we exit 0 with a clear log line so a Kubernetes CronJob doesn't
# alert on absent configuration.
#
# Primary uploader: AWS CLI. Fallback: rclone (see scripts/backup.sh header).
#
# Required env (when SurrealDB is in use):
#   SURREAL_URL               ws://host:8000  or  https://host:8000
#   SURREAL_USER, SURREAL_PASS  credentials passed to `surreal export`
#   SURREAL_NS, SURREAL_DB    namespace + database to export
#   BACKUP_S3_URI             s3://bucket/prefix
# Optional env:
#   BACKUP_RETENTION_DAYS     default 30
#   IRONFLYER_BACKUP_TOOL     "aws" (default) | "rclone"
#   BACKUP_S3_BACKEND         "aws" (default) | "spaces" | "r2" | "minio".
#                             See scripts/backup.sh header for full table;
#                             sets AWS_ENDPOINT_URL_S3 so AWS CLI v2 lands
#                             at the right S3-compatible host.

set -euo pipefail

log() { printf '[backup-surreal] %s %s\n' "$(date -u +%FT%TZ)" "$*"; }
die() { printf '[backup-surreal] ERROR %s %s\n' "$(date -u +%FT%TZ)" "$*" >&2; exit 1; }

# Mirror scripts/backup.sh: route AWS CLI v2 to the right S3-compatible
# endpoint when the operator picks a non-AWS backend.
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

if [ -z "${SURREAL_URL:-}" ]; then
  log "SURREAL_URL is unset — surreal not configured, skipping cleanly"
  exit 0
fi

: "${BACKUP_S3_URI:?BACKUP_S3_URI is required when SURREAL_URL is set}"
: "${SURREAL_USER:?SURREAL_USER is required}"
: "${SURREAL_PASS:?SURREAL_PASS is required}"
: "${SURREAL_NS:?SURREAL_NS is required}"
: "${SURREAL_DB:?SURREAL_DB is required}"

RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-30}"
TOOL="${IRONFLYER_BACKUP_TOOL:-aws}"
TS="$(date -u +%Y%m%d-%H%M%S)"
FILENAME="ironflyer-surreal-${TS}.surql"
TMPDIR="$(mktemp -d)"
LOCAL="${TMPDIR}/${FILENAME}"

trap 'rm -rf "$TMPDIR"' EXIT

case "$TOOL" in
  aws)    command -v aws    >/dev/null 2>&1 || die "aws CLI not found";;
  rclone) command -v rclone >/dev/null 2>&1 || die "rclone not found";;
  *)      die "unknown IRONFLYER_BACKUP_TOOL=$TOOL";;
esac

command -v surreal >/dev/null 2>&1 || die "surreal CLI not found in PATH"

log "exporting SurrealDB ns=${SURREAL_NS} db=${SURREAL_DB} → ${LOCAL}"
surreal export \
  --endpoint "$SURREAL_URL" \
  --username "$SURREAL_USER" \
  --password "$SURREAL_PASS" \
  --namespace "$SURREAL_NS" \
  --database "$SURREAL_DB" \
  "$LOCAL"

BYTES="$(wc -c <"$LOCAL" | tr -d ' ')"
log "export complete: ${BYTES} bytes"

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
  aws s3 ls "${BACKUP_S3_URI%/}/" | while read -r line; do
    [ -z "$line" ] && continue
    obj_date="$(echo "$line" | awk '{print $1" "$2}')"
    obj_name="$(echo "$line" | awk '{print $4}')"
    [ -z "$obj_name" ] && continue
    case "$obj_name" in
      ironflyer-surreal-*.surql) ;;
      *) continue ;;
    esac
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
    --include 'ironflyer-surreal-*.surql'
fi

log "done — surreal backup ${FILENAME} stored at ${DEST}"
