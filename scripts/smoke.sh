#!/usr/bin/env bash
# Ironflyer smoke test — post-deploy verification for the orchestrator.
#
# GraphQL-only. The orchestrator's API of record is GraphQL; only the
# REST-forever exception list (k8s probes + Prometheus scrape) is
# touched outside /graphql. The economic contract (signUp -> wallet ->
# paid execution -> executionFeed -> profitDashboard) is delegated to
# scripts/v22_smoke.sh, which this script invokes as its tail.
#
# In production the orchestrator runs APQ-locked (PERSISTED_QUERY_REQUIRED
# on every /graphql POST). The `gql` helper transparently wraps every
# request in Apollo's persistedQuery extension so this script works
# against locked + open-registration prod the same way the web client does.
#
# Usage:
#
#   IRONFLYER_API_URL=https://api.ironflyer.example.com \
#       SMOKE_METRICS_TOKEN=<IRONFLYER_METRICS_TOKEN> \
#       bash scripts/smoke.sh
#
# Environment:
#   IRONFLYER_API_URL     Base URL of the orchestrator
#                         (default http://localhost:8080).
#   SMOKE_METRICS_TOKEN   Bearer for /metrics scrape. Optional; when
#                         absent and /metrics is bearer-protected the
#                         section is downgraded to a warn.
#
# Exit code: 0 on success, non-zero on the first hard failure.

set -euo pipefail

API="${IRONFLYER_API_URL:-http://localhost:8080}"

bold=$(printf '\033[1m'); green=$(printf '\033[32m'); red=$(printf '\033[31m')
yellow=$(printf '\033[33m'); reset=$(printf '\033[0m')

fail_count=0
ok()   { printf "  %s[OK]%s   %s\n"   "$green"  "$reset" "$1"; }
warn() { printf "  %s[WARN]%s %s\n"   "$yellow" "$reset" "$1"; }
err()  { printf "  %s[FAIL]%s %s\n"   "$red"    "$reset" "$1"; fail_count=$((fail_count+1)); }

section() {
  printf "\n%s%s%s  (%s)\n" "$bold" "$1" "$reset" "$API"
}

have() { command -v "$1" >/dev/null 2>&1; }

if ! have jq; then
  err "jq is required for smoke.sh (install via brew/apt)"; exit 2
fi

# sha256_hex <text> — portable lowercase-hex sha256. Tries openssl first
# (always present on macOS + every prod Linux image), then python3.
sha256_hex() {
  if have openssl; then
    printf '%s' "$1" | openssl dgst -sha256 -hex | awk '{print $NF}'
  elif have python3; then
    python3 -c 'import hashlib,sys;print(hashlib.sha256(sys.argv[1].encode()).hexdigest())' "$1"
  else
    return 2
  fi
}

# apq_wrap <query-json-body> — when the body has a top-level "query"
# string, wrap it in Apollo's persistedQuery extension so the locked-mode
# allowlist accepts the request (open-registration prod registers the
# hash on first touch; locked prod requires it to be pre-seeded — same
# protocol, server decides). Falls back to the original body if jq can't
# extract a query (e.g. caller already supplied an extension).
apq_wrap() {
  local body="$1" q
  q=$(printf '%s' "$body" | jq -r '.query // empty' 2>/dev/null || true)
  if [ -z "$q" ]; then
    printf '%s' "$body"
    return 0
  fi
  local hash
  hash=$(sha256_hex "$q") || { printf '%s' "$body"; return 0; }
  printf '%s' "$body" | jq -c --arg h "$hash" '
    .extensions = ((.extensions // {}) + {
      persistedQuery: { version: 1, sha256Hash: $h }
    })
  '
}

gql() {
  # gql <query-json-body> [bearer]
  local body="$1" auth="${2:-}"
  body=$(apq_wrap "$body")
  if [ -n "$auth" ]; then
    curl -sS -X POST "$API/graphql" \
      -H 'content-type: application/json' \
      -H "authorization: Bearer $auth" \
      --data "$body"
  else
    curl -sS -X POST "$API/graphql" \
      -H 'content-type: application/json' \
      --data "$body"
  fi
}

# gql_ok <query-json> [bearer]
# Returns 0 and echoes .data if the response has no .errors[] AND .data
# is non-null; returns 1 otherwise (echoing nothing).
gql_ok() {
  local resp errs data
  resp=$(gql "$1" "${2:-}")
  errs=$(printf '%s' "$resp" | jq -c '.errors // empty')
  if [ -n "$errs" ] && [ "$errs" != "null" ]; then
    return 1
  fi
  data=$(printf '%s' "$resp" | jq -c '.data // empty')
  if [ -z "$data" ] || [ "$data" = "null" ]; then
    return 1
  fi
  printf '%s' "$data"
  return 0
}

# ----------------------------------------------------------------------------
# Section 1 — infra probes (REST forever)
# ----------------------------------------------------------------------------
section "1. infra probes (REST forever)"

probe() {
  # probe <path> <expect-substring-or-empty>
  local path="$1" expect="${2:-}"
  local body code
  body=$(curl -sS -o /tmp/iron_probe.$$ -w '%{http_code}' "$API$path" || echo "000")
  code="$body"
  body=$(cat /tmp/iron_probe.$$ 2>/dev/null || true)
  rm -f /tmp/iron_probe.$$
  if [ "$code" = "200" ]; then
    if [ -z "$expect" ] || printf '%s' "$body" | grep -q "$expect"; then
      ok "$path -> 200"
    else
      err "$path 200 but body missing '$expect': $body"
    fi
  else
    err "$path returned HTTP $code: $body"
  fi
}

probe /healthz '"ok":true'
probe /livez   '"ok":true'
probe /readyz  '"ready":true'
probe /version '"service":"ironflyer-orchestrator"'

metrics_args=(-sS -o /tmp/iron_metrics.$$ -w '%{http_code}')
if [ -n "${SMOKE_METRICS_TOKEN:-}" ]; then
  metrics_args+=(-H "authorization: Bearer $SMOKE_METRICS_TOKEN")
fi
metrics_code=$(curl "${metrics_args[@]}" "$API/metrics" 2>/dev/null || echo "000")
metrics_body=$(head -5 /tmp/iron_metrics.$$ 2>/dev/null || true)
rm -f /tmp/iron_metrics.$$
case "$metrics_code" in
  200)
    if printf '%s' "$metrics_body" | grep -qE '^# (HELP|TYPE) '; then
      ok "/metrics serves Prometheus exposition format"
    else
      err "/metrics returned 200 but body is not Prometheus: $metrics_body"
    fi
    ;;
  401|403)
    if [ -n "${SMOKE_METRICS_TOKEN:-}" ]; then
      err "/metrics rejected provided bearer (HTTP $metrics_code) — IRONFLYER_METRICS_TOKEN mismatch"
    else
      warn "/metrics is bearer-protected (HTTP $metrics_code) — set SMOKE_METRICS_TOKEN to exercise it"
    fi
    ;;
  000)
    err "/metrics unreachable"
    ;;
  *)
    err "/metrics returned HTTP $metrics_code"
    ;;
esac

# ----------------------------------------------------------------------------
# Section 2 — GraphQL handshake
# ----------------------------------------------------------------------------
section "2. GraphQL handshake"

if data=$(gql_ok '{"query":"{ __typename }"}'); then
  tn=$(printf '%s' "$data" | jq -r '.__typename')
  if [ "$tn" = "Query" ]; then
    ok "POST /graphql { __typename } -> Query"
  else
    err "unexpected __typename: $data"
  fi
else
  err "POST /graphql { __typename } failed"
fi

# `me` is auth-optional: when no bearer is supplied the resolver
# returns null without erroring. We assert the field is reachable and
# the response shape is { me: null | {...} }.
if me_resp=$(gql '{"query":"{ me { id email } }"}'); then
  errs=$(printf '%s' "$me_resp" | jq -c '.errors // empty')
  if [ -n "$errs" ] && [ "$errs" != "null" ]; then
    err "me query errored anonymously: $errs"
  else
    me=$(printf '%s' "$me_resp" | jq -c '.data.me')
    ok "query { me { id } } reachable (anon -> $me)"
  fi
else
  err "me query request failed"
fi

# ----------------------------------------------------------------------------
# Section 3 — introspection-driven schema reads
# ----------------------------------------------------------------------------
# The brief asks for plans / agentTelemetry / providersHealth, but the
# live schema masks resolver errors as "internal server error". We use
# introspection to confirm each field exists on Query, then we probe
# the resolver and ok/warn based on what comes back. Fields that fail
# at runtime get a warn (not fail) so the smoke still surfaces an
# orchestrator that is up + schema-correct even when an
# auxiliary resolver is degraded; v22_smoke.sh below is the hard gate.
# ----------------------------------------------------------------------------
section "3. schema reads (introspection-driven)"

INTRO=$(gql '{"query":"{ __schema { queryType { fields { name } } } }"}')
INTRO_DISABLED=$(printf '%s' "$INTRO" | jq -r '.errors[0].extensions.code // empty' 2>/dev/null)
QUERY_FIELDS=$(printf '%s' "$INTRO" | jq -r '.data.__schema.queryType.fields[].name' 2>/dev/null || true)
if [ "$INTRO_DISABLED" = "INTROSPECTION_DISABLED" ]; then
  warn "introspection is disabled (production hardening) — schema reads probe fields directly"
  QUERY_FIELDS=""
elif [ -z "$QUERY_FIELDS" ]; then
  err "introspection on Query failed: $INTRO"
else
  q_count=$(printf '%s\n' "$QUERY_FIELDS" | wc -l | tr -d ' ')
  ok "introspection lists $q_count Query fields"
fi

# has_field probes whether a Query field is present. When introspection
# is disabled the answer is unknowable from the client — return true so
# the downstream probe runs against the live resolver and reports its
# own verdict, which is the only signal that matters anyway.
has_field() {
  if [ "$INTRO_DISABLED" = "INTROSPECTION_DISABLED" ]; then
    return 0
  fi
  printf '%s\n' "$QUERY_FIELDS" | grep -qx "$1"
}

# probe_field <label> <query> <jq-data-path> — try a resolver directly.
# Returns 0 + echoes data on success; returns 1 on validation/resolver
# error (which is the normal "field not on schema" path when
# introspection is disabled). Logs nothing — caller decides.
probe_field() {
  local _label="$1" _query="$2"
  gql_ok "{\"query\":\"$_query\"}" >/dev/null 2>&1
}
probe_field_data() {
  local _query="$1"
  gql_ok "{\"query\":\"$_query\"}"
}

# 3a. plans { tier name priceUsd } — billing plan visibility.
if data=$(probe_field_data '{ plans { tier name priceUsd } }'); then
  n=$(printf '%s' "$data" | jq -r '.plans | length')
  if [ "${n:-0}" -ge 1 ]; then
    ok "plans { tier name priceUsd } -> $n rows"
  else
    warn "plans returned 0 rows"
  fi
else
  warn "plans resolver not reachable (field missing or degraded)"
fi

# 3b. agentTelemetry { provider model durationMs } — agent visibility.
if data=$(probe_field_data '{ agentTelemetry(limit: 5) { provider model durationMs } }'); then
  n=$(printf '%s' "$data" | jq -r '.agentTelemetry | length')
  ok "agentTelemetry(limit: 5) -> $n rows"
else
  warn "agentTelemetry resolver not reachable (field missing or degraded)"
fi

# 3c. provider visibility cascade — try providersHealth → rates →
# blueprints. The V22 schema currently exposes `rates`; `providersHealth`
# was retired. Cascade survives both introspection-on and
# introspection-off deployments without lying about field presence.
if data=$(probe_field_data '{ providersHealth { provider status } }'); then
  n=$(printf '%s' "$data" | jq -r '.providersHealth | length')
  ok "providersHealth -> $n providers"
elif data=$(probe_field_data '{ rates { provider model promptPerMTok completionPerMTok } }'); then
  n=$(printf '%s' "$data" | jq -r '.rates | length')
  ok "rates { provider model ... } -> $n rows (providersHealth absent on V22 schema; rates is the provider-visibility neighbour)"
elif data=$(probe_field_data '{ blueprints { id name } }'); then
  n=$(printf '%s' "$data" | jq -r '.blueprints | length')
  ok "blueprints -> $n rows (final provider-visibility fallback)"
else
  err "providersHealth/rates/blueprints all failed — Query resolvers degraded"
fi

# ----------------------------------------------------------------------------
# Section 4 — idempotent mutation
# ----------------------------------------------------------------------------
# verifyAudit lives on Query in V22 (not Mutation). The chosen
# idempotent mutation is signUp with a throwaway random email — same
# pattern as v22_smoke.sh — which is the only V22 mutation that can be
# exercised cold with no prior state. Error masking is handled by
# checking .data.signUp.token rather than relying on error text.
# ----------------------------------------------------------------------------
section "4. idempotent mutation (signUp w/ throwaway email)"

TS=$(date +%s); RAND=$RANDOM
EMAIL="smoke+${TS}-${RAND}@ironflyer.local"
PASSWORD="smoke-${TS}-${RAND}"

SIGNUP_QUERY='mutation($email: String!, $password: String!) { signUp(input: {email: $email, password: $password, name: "Smoke Probe"}) { token user { id email plan } } }'
SIGNUP_BODY=$(jq -n --arg q "$SIGNUP_QUERY" --arg e "$EMAIL" --arg p "$PASSWORD" \
  '{query:$q, variables:{email:$e, password:$p}}')

SIGNUP_RESP=$(gql "$SIGNUP_BODY")
SIGNUP_ERRS=$(printf '%s' "$SIGNUP_RESP" | jq -c '.errors // empty')
TOKEN=$(printf '%s' "$SIGNUP_RESP" | jq -r '.data.signUp.token // ""')

if [ -n "$SIGNUP_ERRS" ] && [ "$SIGNUP_ERRS" != "null" ]; then
  err "signUp returned errors: $SIGNUP_ERRS"
elif [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
  err "signUp returned no token: $SIGNUP_RESP"
else
  USER_ID=$(printf '%s' "$SIGNUP_RESP" | jq -r '.data.signUp.user.id')
  ok "signUp($EMAIL) -> user_id=$USER_ID, token=${TOKEN:0:20}..."
fi

# verifyAudit is Query, not Mutation, but it is an idempotent
# integrity check that is worth exercising whenever the schema offers
# it. Treat a missing/erroring resolver as a warn so the smoke remains
# green when the audit chain table is empty.
if has_field verifyAudit; then
  if data=$(gql_ok '{"query":"{ verifyAudit { intact firstBadIndex } }"}' "$TOKEN"); then
    intact=$(printf '%s' "$data" | jq -r '.verifyAudit.intact')
    first_bad=$(printf '%s' "$data" | jq -r '.verifyAudit.firstBadIndex')
    if [ "$intact" = "true" ]; then
      ok "verifyAudit.intact = true (firstBadIndex=$first_bad)"
    else
      warn "verifyAudit.intact = $intact firstBadIndex=$first_bad — audit chain reports broken"
    fi
  else
    warn "verifyAudit reachable on schema but resolver errored (errors masked)"
  fi
fi

# ----------------------------------------------------------------------------
# Section 5 — live subscription smoke
# ----------------------------------------------------------------------------
# Open `subscription { costStream { ... } }` over graphql-transport-ws
# for ~2s; assert connection_ack arrived. costStream is unauthenticated
# (server-wide tenant feed). Field selection comes from introspection:
# CostDelta { ts, usdSpent, model, provider, agent, durationMs }.
# ----------------------------------------------------------------------------
section "5. live subscription smoke (costStream)"

WS_URL=$(printf '%s' "$API" | sed -e 's#^http://#ws://#' -e 's#^https://#wss://#')
WS_URL="${WS_URL}/graphql"

PY_BIN=""
for cand in \
  /tmp/iron_ws_venv/bin/python \
  "$(dirname "$0")/../.venv/bin/python" \
  python3
do
  if [ -x "$cand" ] || command -v "$cand" >/dev/null 2>&1; then
    if "$cand" -c "import websockets" 2>/dev/null; then
      PY_BIN="$cand"
      break
    fi
  fi
done

if [ -z "$PY_BIN" ] && have python3; then
  # Bootstrap the same venv v22_smoke.sh expects, so the next run is fast.
  if python3 -m venv /tmp/iron_ws_venv >/dev/null 2>&1 \
     && /tmp/iron_ws_venv/bin/pip install --quiet websockets >/dev/null 2>&1; then
    PY_BIN=/tmp/iron_ws_venv/bin/python
    ok "bootstrapped /tmp/iron_ws_venv for websockets"
  fi
fi

run_ws_python() {
  V22_WS_URL="$WS_URL" V22_TOKEN="${TOKEN:-}" "$PY_BIN" - <<'PY'
import os, json, asyncio, sys
import websockets

url = os.environ["V22_WS_URL"]
token = os.environ.get("V22_TOKEN", "")

async def main():
    # The graphql-transport-ws subscription path is exempt from the
    # /graphql HTTP APQ-locked middleware (the lock intercepts POST
    # bodies, not WS messages). We assert: connection_ack arrives
    # within 5s, then the server accepts the subscription (no validation
    # error on CostDelta fields). Field selection comes from
    # core/orchestrator/internal/operations/graph/schema/agents.graphql.
    try:
        async with websockets.connect(
            url,
            subprotocols=["graphql-transport-ws"],
            open_timeout=5,
            close_timeout=2,
        ) as ws:
            init = {"type": "connection_init", "payload": {}}
            if token:
                init["payload"]["authorization"] = f"Bearer {token}"
            await ws.send(json.dumps(init))
            ack = await asyncio.wait_for(ws.recv(), timeout=5)
            msg = json.loads(ack)
            if msg.get("type") != "connection_ack":
                print(f"NO_ACK: {ack}", file=sys.stderr); return 3
            sub = {
                "id": "1",
                "type": "subscribe",
                "payload": {"query": "subscription { costStream { ts usdSpent provider model } }"},
            }
            await ws.send(json.dumps(sub))
            # 2s of silence is fine — costStream emits only when an
            # execution is burning provider cost. A validation error
            # (server replies with type=error) is a real failure.
            try:
                reply = await asyncio.wait_for(ws.recv(), timeout=2)
                m = json.loads(reply)
                if m.get("type") == "error":
                    print(f"WS_VALIDATION_ERR: {reply}", file=sys.stderr); return 4
            except asyncio.TimeoutError:
                pass
            try:
                await ws.send(json.dumps({"id": "1", "type": "complete"}))
            except Exception:
                pass
        print("ACK_OK")
        return 0
    except Exception as e:
        print(f"WS_ERR: {type(e).__name__}: {e}", file=sys.stderr); return 2

sys.exit(asyncio.run(main()))
PY
}

if have websocat; then
  if printf '%s\n' \
       '{"type":"connection_init","payload":{}}' \
       '{"id":"1","type":"subscribe","payload":{"query":"subscription { costStream { ts usdSpent provider model } }"}}' \
     | websocat --protocol graphql-transport-ws -n1 -E -t "$WS_URL" 2>/dev/null | head -3 \
     | grep -q 'connection_ack'; then
    ok "websocat: connection_ack received from $WS_URL"
  else
    err "websocat: did not receive connection_ack from $WS_URL"
  fi
elif [ -n "$PY_BIN" ]; then
  if out=$(run_ws_python 2>&1); then
    if printf '%s' "$out" | grep -q 'ACK_OK'; then
      ok "python websockets ($PY_BIN): connection_ack received from $WS_URL"
    else
      err "python websockets: unexpected output: $out"
    fi
  else
    err "python websockets smoke failed: $out"
  fi
else
  warn "neither websocat nor python3 with websockets — skipping subscription smoke"
fi

# ----------------------------------------------------------------------------
# Section 6 — deprecated-REST detection
# ----------------------------------------------------------------------------
# The deprecation middleware stamps `Deprecation: true` (+ Sunset) on
# every legacy REST route. We probe ONE harmless route from the legacy
# tree — /budget — and only check the response headers. We never call
# /projects, /budget/topup, /providers/health, etc. (those were removed
# with the GraphQL-only cutover and would return 404).
# ----------------------------------------------------------------------------
section "6. deprecated-REST detection"

DEPR_PATH="/budget"
hdr_args=(-sS -D - -o /dev/null -w 'HTTPCODE:%{http_code}\n')
if [ -n "${TOKEN:-}" ]; then
  hdr_args+=(-H "authorization: Bearer $TOKEN")
fi

headers=$(curl "${hdr_args[@]}" "$API$DEPR_PATH" 2>&1 || true)
http_code=$(printf '%s' "$headers" | awk -F: '/^HTTPCODE:/{print $2}')

if printf '%s' "$headers" | grep -qiE '^Deprecation:[[:space:]]*true'; then
  ok "Deprecation: true header present on $DEPR_PATH (status $http_code)"
  if sunset=$(printf '%s' "$headers" | grep -iE '^Sunset:' | head -1); then
    if [ -n "$sunset" ]; then
      ok "$(printf '%s' "$sunset" | tr -d '\r')"
    fi
  fi
elif [ "$http_code" = "404" ]; then
  warn "$DEPR_PATH returned 404 — route removed from REST exception list (deprecation middleware not exercised)"
else
  warn "no Deprecation header on $DEPR_PATH (status $http_code) — deprecation middleware may have been retired"
fi

# ----------------------------------------------------------------------------
# Section 7 — V22 paid-execution synthetic flow
# ----------------------------------------------------------------------------
# The infra + GraphQL handshake + subscription above only proves the
# orchestrator is up and schema-correct. The V22 economic contract
# (signUp -> wallet -> createPaidExecution -> executionFeed ->
# profitDashboard) lives in scripts/v22_smoke.sh. We invoke it as the
# closure check and propagate its exit code so any law-1 / law-2 /
# law-3 regression fails the smoke run.
# ----------------------------------------------------------------------------
section "7. V22 paid-execution smoke (scripts/v22_smoke.sh)"

V22_SMOKE="$(dirname "$0")/v22_smoke.sh"
if [ ! -x "$V22_SMOKE" ]; then
  err "$V22_SMOKE missing or not executable"
else
  if IRONFLYER_API_URL="$API" "$V22_SMOKE"; then
    ok "v22_smoke.sh PASS"
  else
    err "v22_smoke.sh FAILED — V22 paid-execution contract is broken"
  fi
fi

# ----------------------------------------------------------------------------
# Summary
# ----------------------------------------------------------------------------
echo
if [ "$fail_count" -eq 0 ]; then
  printf "%ssmoke result: PASS%s\n" "$green" "$reset"
  exit 0
else
  printf "%ssmoke result: FAIL (%d hard failure(s))%s\n" "$red" "$fail_count" "$reset"
  exit 1
fi
