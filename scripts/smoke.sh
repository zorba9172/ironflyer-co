#!/usr/bin/env bash
# Ironflyer smoke test. Hits every critical endpoint with curl + jq and
# exits non-zero on the first failure. Run after `helm upgrade --install`
# to verify a deployment, or locally after `go run ./apps/orchestrator/...`.
#
#   ORCHESTRATOR=http://localhost:8080 RUNTIME=http://localhost:8090 \
#   WEB=http://localhost:3000 ./scripts/smoke.sh
#
# Optional auth: export IRONFLYER_TOKEN=eyJ... to exercise authenticated
# endpoints. Without it, only the public catalogue routes are tested.
set -euo pipefail

ORCHESTRATOR="${ORCHESTRATOR:-http://localhost:8080}"
RUNTIME="${RUNTIME:-http://localhost:8090}"
WEB="${WEB:-http://localhost:3000}"
TOKEN="${IRONFLYER_TOKEN:-}"

pass=0
fail=0
warn=0

bold='\033[1m'
green='\033[32m'
red='\033[31m'
yellow='\033[33m'
reset='\033[0m'

check() {
  local name="$1"; shift
  local out
  if out=$("$@" 2>&1); then
    pass=$((pass+1))
    printf "${green}✓${reset} %s\n" "$name"
  else
    fail=$((fail+1))
    printf "${red}✗${reset} %s\n    %s\n" "$name" "$out"
  fi
}

soft() {
  local name="$1"; shift
  local out
  if out=$("$@" 2>&1); then
    pass=$((pass+1))
    printf "${green}✓${reset} %s\n" "$name"
  else
    warn=$((warn+1))
    printf "${yellow}!${reset} %s (warning)\n    %s\n" "$name" "$out"
  fi
}

# ------------- Orchestrator -----------------------------------------------
orch_health() {
  curl -fsS "$ORCHESTRATOR/health" | grep -q '"ok":true'
}
orch_plans() {
  curl -fsS "$ORCHESTRATOR/budget/plans" | grep -q '"tier"'
}
orch_rates() {
  curl -fsS "$ORCHESTRATOR/budget/rates" | grep -q 'provider'
}
orch_agents() {
  if [ -z "$TOKEN" ]; then return 0; fi
  curl -fsS -H "Authorization: Bearer $TOKEN" "$ORCHESTRATOR/agents" | grep -q '"planner"' || \
    curl -fsS -H "Authorization: Bearer $TOKEN" "$ORCHESTRATOR/agents" | grep -q 'planner'
}
orch_stripe_disabled_when_unset() {
  # If STRIPE_SECRET_KEY isn't set, /budget/webhook must 503. We can't easily
  # know which mode the server is in from outside, so we treat both 200-style
  # responses to GET (always 404 because route is POST) as acceptable. The
  # checkout route requires auth + body; just make sure POST to webhook
  # without signature returns 4xx/503.
  local code
  code=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "$ORCHESTRATOR/budget/webhook" -H 'content-type: application/json' -d '{}')
  [ "$code" -ge 400 ]
}

printf "${bold}orchestrator${reset}  %s\n" "$ORCHESTRATOR"
check "health"            orch_health
check "budget/plans"      orch_plans
check "budget/rates"      orch_rates
check "agents (auth)"     orch_agents
check "webhook rejects unsigned" orch_stripe_disabled_when_unset

# ------------- Runtime -----------------------------------------------------
rt_health() {
  curl -fsS "$RUNTIME/health" | grep -q '"ok":true'
}
rt_workspaces_requires_auth() {
  # When auth is enabled the runtime 401s; when AuthOptional, 200. Both fine.
  local code
  code=$(curl -sS -o /dev/null -w '%{http_code}' "$RUNTIME/workspaces")
  [ "$code" -eq 200 ] || [ "$code" -eq 401 ]
}

printf "${bold}runtime${reset}       %s\n" "$RUNTIME"
check "health"               rt_health
check "workspaces reachable" rt_workspaces_requires_auth

# ------------- Web ---------------------------------------------------------
web_home() {
  local code
  code=$(curl -sS -o /dev/null -w '%{http_code}' "$WEB/")
  [ "$code" -lt 400 ]
}
web_pricing() {
  local code
  code=$(curl -sS -o /dev/null -w '%{http_code}' "$WEB/pricing")
  [ "$code" -lt 400 ]
}

printf "${bold}web${reset}           %s\n" "$WEB"
soft "/ reachable"        web_home
soft "/pricing reachable" web_pricing

# ------------- Summary -----------------------------------------------------
echo
printf "${bold}smoke result:${reset} %d passed, %d failed, %d warnings\n" "$pass" "$fail" "$warn"
[ "$fail" -eq 0 ]
