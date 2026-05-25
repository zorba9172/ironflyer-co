#!/usr/bin/env bash
# verify-headers.sh — post-deploy verification of the production security
# headers on both the orchestrator API and the web dashboard. Curls each
# surface and greps for the expected response headers; exits non-zero on
# the first miss so it can be wired into a deploy gate (helm test, GitHub
# Actions check, cron).
#
# Usage:
#   IRONFLYER_API_URL=https://api.ironflyer.dev \
#   IRONFLYER_WEB_URL=https://ironflyer.dev \
#     scripts/verify-headers.sh
#
# Defaults probe local dev (orchestrator :8080, web :3000) so the script
# also works as a smoke check after `make dev`.
set -euo pipefail

API_URL="${IRONFLYER_API_URL:-http://localhost:8080}"
WEB_URL="${IRONFLYER_WEB_URL:-http://localhost:3000}"

fail=0

check() {
  # check <surface-label> <url> <header-name> [extra-curl-args...]
  local label="$1" url="$2" header="$3"
  shift 3
  local headers
  if ! headers="$(curl -fsSI "$@" "$url" 2>/dev/null)"; then
    echo "FAIL: ${label} ${url} — curl failed" >&2
    fail=1
    return
  fi
  if printf '%s\n' "$headers" | grep -qi "^${header}:"; then
    echo "ok:   ${label} ${url} ${header}"
  else
    echo "FAIL: ${label} ${url} missing ${header}" >&2
    fail=1
  fi
}

# Orchestrator: /livez is the liveness probe — public, cheap, always on.
# HSTS only when behind TLS (the middleware deliberately omits it on HTTP
# to avoid poisoning local dev), so we only assert it for https URLs.
echo "==> Orchestrator: ${API_URL}/livez"
check "orchestrator" "${API_URL}/livez" "X-Content-Type-Options"
check "orchestrator" "${API_URL}/livez" "X-Frame-Options"
check "orchestrator" "${API_URL}/livez" "Referrer-Policy"
check "orchestrator" "${API_URL}/livez" "Permissions-Policy"
check "orchestrator" "${API_URL}/livez" "Content-Security-Policy"
if [[ "${API_URL}" == https://* ]]; then
  check "orchestrator" "${API_URL}/livez" "Strict-Transport-Security"
fi

# Web: hit the marketing root. Next.js sets headers via next.config.mjs
# headers() — see apps/web/next.config.mjs.
echo "==> Web: ${WEB_URL}/"
check "web" "${WEB_URL}/" "X-Content-Type-Options"
check "web" "${WEB_URL}/" "X-Frame-Options"
check "web" "${WEB_URL}/" "Referrer-Policy"
check "web" "${WEB_URL}/" "Permissions-Policy"
check "web" "${WEB_URL}/" "Content-Security-Policy"
check "web" "${WEB_URL}/" "Strict-Transport-Security"

if [[ "$fail" -ne 0 ]]; then
  echo
  echo "verify-headers: FAILED — one or more headers missing" >&2
  exit 1
fi
echo
echo "verify-headers: all expected headers present"
