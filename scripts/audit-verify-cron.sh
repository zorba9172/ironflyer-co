#!/bin/sh
# audit-verify-cron.sh
#
# Hourly integrity probe for the Ironflyer audit chain.
#
# What it does:
#   1. GETs ${IRONFLYER_API_URL}/audit/verify with bearer ${AUDIT_BEARER}.
#   2. Parses the JSON response for `chain_valid`.
#   3. If chain_valid is false (or the call failed), emits a structured
#      error log line and POSTs a Slack/Discord-shaped webhook payload
#      to ${ALERT_WEBHOOK_URL}.
#   4. Exits non-zero on break so the Kubernetes CronJob is marked failed.
#
# Required env:
#   IRONFLYER_API_URL    e.g. https://api.ironflyer.example.com
#   AUDIT_BEARER         service-account bearer token
#   ALERT_WEBHOOK_URL    Slack/Discord-compatible incoming webhook
#
# Designed to run inside curlimages/curl (busybox sh + curl). No bash, no
# jq — we lean on POSIX sh and a minimal regex parse so the image stays tiny.

set -u

: "${IRONFLYER_API_URL:?IRONFLYER_API_URL is required}"
: "${AUDIT_BEARER:?AUDIT_BEARER is required}"
: "${ALERT_WEBHOOK_URL:?ALERT_WEBHOOK_URL is required}"

URL="${IRONFLYER_API_URL%/}/audit/verify"
TS="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

log_error() {
  # Structured single-line log: zerolog-compatible key=value pairs.
  printf 'time=%s level=error component=audit-verify-cron %s\n' "$TS" "$*" >&2
}

log_info() {
  printf 'time=%s level=info component=audit-verify-cron %s\n' "$TS" "$*"
}

post_alert() {
  msg="$1"
  # Slack/Discord both accept {"text": "..."} on incoming webhooks.
  payload=$(printf '{"text": "Ironflyer audit chain broken at %s — %s"}' "$TS" "$msg")
  curl --silent --show-error --fail \
       --max-time 10 \
       -H 'Content-Type: application/json' \
       -X POST \
       -d "$payload" \
       "$ALERT_WEBHOOK_URL" >/dev/null 2>&1 || \
    log_error msg=\"webhook post failed\"
}

# Fetch the response body + HTTP status. -w prints status on its own line
# after the body so we can split cleanly without temp files.
RESP=$(curl --silent --show-error \
            --max-time 15 \
            -H "Authorization: Bearer $AUDIT_BEARER" \
            -H 'Accept: application/json' \
            -w '\n%{http_code}' \
            "$URL") || {
  log_error msg=\"curl failed\" url="$URL"
  post_alert "audit verify HTTP call failed (network or auth)"
  exit 2
}

STATUS=$(printf '%s' "$RESP" | awk 'END{print}')
BODY=$(printf '%s' "$RESP" | sed '$d')

if [ "$STATUS" != "200" ]; then
  log_error chain_break=true http_status="$STATUS" body="$BODY"
  post_alert "audit verify returned HTTP $STATUS"
  exit 3
fi

# POSIX-grep extract of "chain_valid": true|false. Robust to whitespace
# and field ordering. If the key is missing we treat it as a break.
VALID=$(printf '%s' "$BODY" | grep -oE '"chain_valid"[[:space:]]*:[[:space:]]*(true|false)' | head -n 1 | grep -oE '(true|false)')

if [ -z "$VALID" ]; then
  log_error chain_break=true reason=\"chain_valid missing from response\" body="$BODY"
  post_alert "audit verify response missing chain_valid field"
  exit 4
fi

if [ "$VALID" = "false" ]; then
  # Pull the first broken event id if the orchestrator returned one — it
  # makes the page actionable without forcing the operator to curl again.
  BROKEN_ID=$(printf '%s' "$BODY" | grep -oE '"broken_event_id"[[:space:]]*:[[:space:]]*"[^"]*"' | head -n 1 | sed 's/.*"\([^"]*\)"$/\1/')
  log_error chain_break=true broken_event_id="${BROKEN_ID:-unknown}" body="$BODY"
  post_alert "chain_valid=false; first broken event id: ${BROKEN_ID:-unknown}"
  exit 1
fi

log_info chain_valid=true
exit 0
