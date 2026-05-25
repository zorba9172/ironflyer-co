#!/usr/bin/env bash
# Ironflyer Postgres restore.
#
# Usage:
#   scripts/restore.sh <s3-key-or-local-path> [--dry-run]
#
# If the argument starts with s3:// (or matches a configured rclone remote
# via IRONFLYER_BACKUP_TOOL=rclone), the dump is downloaded to a temp dir
# before restore. Otherwise it is treated as a local file path.
#
# Restore command:
#   pg_restore --clean --if-exists --no-owner --dbname="$POSTGRES_URL" <dump>
#
# SAFETY:
#   Restoring on top of a live production DB is destructive. This script
#   refuses to run unless ${ALLOW_RESTORE_HOST_REGEX} is set AND the host
#   embedded in ${POSTGRES_URL} matches that regex. Operators MUST opt in
#   per-environment (e.g. ALLOW_RESTORE_HOST_REGEX='^(localhost|.*\.staging\.)$').
#
# Required env:
#   POSTGRES_URL              target DSN
#   ALLOW_RESTORE_HOST_REGEX  regex the DSN host must satisfy
# Optional env:
#   BACKUP_S3_BACKEND         "aws" (default) | "spaces" | "r2" | "minio".
#                             Same semantics as scripts/backup.sh — exports
#                             AWS_ENDPOINT_URL_S3 so `aws s3 cp` reaches the
#                             right S3-compatible host:
#                               spaces : https://${DO_SPACES_REGION}.digitaloceanspaces.com
#                                        (DO_SPACES_REGION defaults to nyc3)
#                               r2     : https://${R2_ACCOUNT_ID}.r2.cloudflarestorage.com
#                               minio  : ${MINIO_ENDPOINT}

set -euo pipefail

log()  { printf '[restore] %s %s\n' "$(date -u +%FT%TZ)" "$*"; }
warn() { printf '[restore] WARN %s %s\n' "$(date -u +%FT%TZ)" "$*" >&2; }
die()  { printf '[restore] ERROR %s %s\n' "$(date -u +%FT%TZ)" "$*" >&2; exit 1; }

# Route AWS CLI v2 to the right S3-compatible endpoint based on the
# operator's backend choice. See scripts/backup.sh header for the full
# table; these branches are identical so a restore-from-Spaces uses the
# same env shape as the backup that produced the dump.
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

DRY_RUN=0
SRC=""
for arg in "$@"; do
  case "$arg" in
    --dry-run) DRY_RUN=1 ;;
    -h|--help)
      sed -n '2,22p' "$0"
      exit 0
      ;;
    --*) die "unknown flag: $arg" ;;
    *)
      [ -n "$SRC" ] && die "only one source argument supported (got '$SRC' and '$arg')"
      SRC="$arg"
      ;;
  esac
done

[ -n "$SRC" ] || die "missing source argument (s3 key, s3:// URI, or local path)"
: "${POSTGRES_URL:?POSTGRES_URL is required}"
: "${ALLOW_RESTORE_HOST_REGEX:?ALLOW_RESTORE_HOST_REGEX is required — operator must opt in to which hosts may be restored}"

# Extract host out of postgres://user:pass@host:port/db?...
HOST_PART="${POSTGRES_URL#*://}"
HOST_PART="${HOST_PART#*@}"
HOST="${HOST_PART%%[:/?]*}"
[ -n "$HOST" ] || die "could not parse host from POSTGRES_URL"

if ! printf '%s' "$HOST" | grep -Eq "$ALLOW_RESTORE_HOST_REGEX"; then
  die "refusing to restore: host '$HOST' does not match ALLOW_RESTORE_HOST_REGEX='$ALLOW_RESTORE_HOST_REGEX'"
fi
log "host '$HOST' matches ALLOW_RESTORE_HOST_REGEX — proceeding"

TOOL="${IRONFLYER_BACKUP_TOOL:-aws}"
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

LOCAL=""
case "$SRC" in
  s3://*)
    REMOTE="$SRC"
    LOCAL="${TMPDIR}/$(basename "$SRC")"
    if [ "$DRY_RUN" -eq 1 ]; then
      log "DRY-RUN: would download $REMOTE → $LOCAL"
    else
      log "downloading $REMOTE → $LOCAL"
      if [ "$TOOL" = "aws" ]; then
        command -v aws >/dev/null 2>&1 || die "aws CLI not found"
        aws s3 cp "$REMOTE" "$LOCAL" --no-progress
      else
        command -v rclone >/dev/null 2>&1 || die "rclone not found"
        rclone copyto "$REMOTE" "$LOCAL"
      fi
    fi
    ;;
  *)
    if [ -f "$SRC" ]; then
      LOCAL="$SRC"
      log "using local dump: $LOCAL"
    else
      # treat as bare key relative to BACKUP_S3_URI when set
      if [ -n "${BACKUP_S3_URI:-}" ]; then
        REMOTE="${BACKUP_S3_URI%/}/$SRC"
        LOCAL="${TMPDIR}/$(basename "$SRC")"
        if [ "$DRY_RUN" -eq 1 ]; then
          log "DRY-RUN: would download $REMOTE → $LOCAL"
        else
          log "interpreting '$SRC' as key under BACKUP_S3_URI → $REMOTE"
          if [ "$TOOL" = "aws" ]; then
            command -v aws >/dev/null 2>&1 || die "aws CLI not found"
            aws s3 cp "$REMOTE" "$LOCAL" --no-progress
          else
            command -v rclone >/dev/null 2>&1 || die "rclone not found"
            rclone copyto "$REMOTE" "$LOCAL"
          fi
        fi
      else
        die "source '$SRC' is neither s3:// URI, an existing file, nor a key under BACKUP_S3_URI"
      fi
    fi
    ;;
esac

if [ "$DRY_RUN" -eq 1 ]; then
  log "DRY-RUN: would run pg_restore --clean --if-exists --no-owner against host '$HOST' using dump '${LOCAL:-<downloaded>}'"
  exit 0
fi

command -v pg_restore >/dev/null 2>&1 || die "pg_restore not found in PATH"

log "restoring → host=$HOST dump=$LOCAL"
pg_restore \
  --clean --if-exists --no-owner \
  --dbname="$POSTGRES_URL" \
  "$LOCAL"

log "restore complete"
