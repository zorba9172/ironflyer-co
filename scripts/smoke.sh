#!/usr/bin/env bash
# Ironflyer smoke test — post-deploy verification for the orchestrator.
#
# The orchestrator's API of record is GraphQL. Legacy REST is deprecated and
# sunsets 2026-08-01; this smoke script exercises GraphQL for product reads +
# writes and only touches REST for the routes that are REST forever:
# k8s probes (/livez, /readyz, /version), Prometheus scrape (/metrics), and
# one deprecated route to confirm the deprecation middleware is wired.
#
# Usage:
#
#   IRONFLYER_API_URL=https://api.ironflyer.example.com \
#   SMOKE_BEARER=$(./scripts/issue-smoke-jwt.sh) \
#       ./scripts/smoke.sh
#
# Environment:
#   IRONFLYER_API_URL  Base URL of the orchestrator (default http://localhost:8080).
#   SMOKE_BEARER       JWT for a smoke-test user. Optional; auth-gated sections
#                      warn-skip when unset so the script still runs in dev.
#
# Exit code: 0 on success, non-zero on the first hard failure.

set -euo pipefail

API="${IRONFLYER_API_URL:-http://localhost:8080}"
BEARER="${SMOKE_BEARER:-}"

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

# json_extract <json> <jq-path>
# Uses jq when available, falls back to python3 for environments without jq.
json_extract() {
  local body="$1" path="$2"
  if have jq; then
    printf '%s' "$body" | jq -r "$path"
  elif have python3; then
    IRONFLYER_SMOKE_JSON="$body" IRONFLYER_SMOKE_PATH="$path" python3 - <<'PY'
import json, os
path = os.environ.get('IRONFLYER_SMOKE_PATH', '').strip()
raw = os.environ.get('IRONFLYER_SMOKE_JSON', '')
try:
    data = json.loads(raw)
except Exception:
    print('')
    raise SystemExit(0)
# Translate a tiny subset of jq paths: .a.b.c, .a[0].b, .a | length
def walk(d, p):
    p = p.strip().lstrip('.')
    if p.endswith('| length'):
        return len(walk(d, p[:-len('| length')].strip()))
    cur = d
    if not p:
        return cur
    for part in p.split('.'):
        if '[' in part:
            name, idx = part.split('[', 1)
            idx = int(idx.rstrip(']'))
            if name:
                cur = cur[name]
            cur = cur[idx]
        else:
            cur = cur[part]
    return cur
try:
    v = walk(data, path)
except Exception:
    print('')
    raise SystemExit(0)
if isinstance(v, (dict, list)):
    print(json.dumps(v))
else:
    print('' if v is None else v)
PY
  else
    # Last resort — caller must do its own grep.
    printf '%s' "$body"
  fi
}

gql_post() {
  # gql_post <query-json-body> [auth]
  local body="$1" auth="${2:-}"
  if [ -n "$auth" ]; then
    curl -fsS -X POST "$API/graphql" \
      -H 'content-type: application/json' \
      -H "authorization: Bearer $auth" \
      --data "$body"
  else
    curl -fsS -X POST "$API/graphql" \
      -H 'content-type: application/json' \
      --data "$body"
  fi
}

# ----------------------------------------------------------------------------
# Section 1 — infra probes (REST forever)
# ----------------------------------------------------------------------------
section "1. infra probes"

if body=$(curl -fsS "$API/livez" 2>&1); then
  if printf '%s' "$body" | grep -q '"ok":true'; then
    ok "/livez returns ok:true"
  else
    err "/livez missing ok:true ($body)"
  fi
else
  err "/livez unreachable: $body"
fi

if curl -fsS -o /dev/null "$API/readyz"; then
  ok "/readyz returns 200"
else
  err "/readyz unreachable or non-200"
fi

if body=$(curl -fsS "$API/version" 2>&1); then
  build_id=$(json_extract "$body" '.version' 2>/dev/null || true)
  if [ -z "$build_id" ] || [ "$build_id" = "null" ]; then
    build_id=$(printf '%s' "$body" | tr -d '\n')
  fi
  ok "/version build=${build_id}"
else
  err "/version unreachable: $body"
fi

if metrics_head=$(curl -fsS "$API/metrics" 2>&1 | head -5); then
  if printf '%s' "$metrics_head" | grep -qE '^# (HELP|TYPE) '; then
    ok "/metrics serves Prometheus exposition format"
  else
    err "/metrics did not return a Prometheus header (got: $metrics_head)"
  fi
else
  err "/metrics unreachable"
fi

# ----------------------------------------------------------------------------
# Section 2 — GraphQL handshake
# ----------------------------------------------------------------------------
section "2. GraphQL handshake"

if body=$(gql_post '{"query":"{ __typename }"}'); then
  tn=$(json_extract "$body" '.data.__typename')
  if [ "$tn" = "Query" ]; then
    ok "POST /graphql { __typename } -> Query"
  else
    err "Unexpected __typename: $body"
  fi
else
  err "POST /graphql failed (server down or schema broken)"
fi

if [ -n "$BEARER" ]; then
  if body=$(gql_post '{"query":"query Me { me { id email } }"}' "$BEARER"); then
    me_id=$(json_extract "$body" '.data.me.id')
    if [ -n "$me_id" ] && [ "$me_id" != "null" ]; then
      ok "query Me { me { id } } -> $me_id"
    else
      err "query Me returned empty id: $body"
    fi
  else
    err "query Me request failed"
  fi
else
  warn "SMOKE_BEARER unset — skipping query Me (dev mode)"
fi

# ----------------------------------------------------------------------------
# Section 3 — sample read operations
# ----------------------------------------------------------------------------
section "3. sample reads"

if body=$(gql_post '{"query":"query ListPlans { plans { tier monthlyUsd } }"}'); then
  count=$(json_extract "$body" '.data.plans | length' 2>/dev/null || echo 0)
  if [ "${count:-0}" -ge 1 ] 2>/dev/null; then
    ok "ListPlans returned ${count} plans"
  else
    err "ListPlans returned 0 plans: $body"
  fi
else
  err "ListPlans request failed"
fi

if body=$(gql_post '{"query":"query ProvidersHealth { services { name status } }"}'); then
  # Validate every row has both name + status. We accept an empty list (dev)
  # but flag any row missing either field.
  if have jq; then
    bad=$(printf '%s' "$body" | jq -r '.data.services // [] | map(select((.name|not) or (.status|not))) | length')
    total=$(printf '%s' "$body" | jq -r '.data.services // [] | length')
    if [ "$bad" = "0" ]; then
      ok "ProvidersHealth: ${total} services, all rows have name+status"
    else
      err "ProvidersHealth: ${bad}/${total} rows missing name or status"
    fi
  else
    # Fallback — just confirm the field appears
    if printf '%s' "$body" | grep -q '"services"'; then
      ok "ProvidersHealth returned services field (jq missing — skipped row validation)"
    else
      err "ProvidersHealth missing services field"
    fi
  fi
else
  err "ProvidersHealth request failed"
fi

if [ -n "$BEARER" ]; then
  if body=$(gql_post '{"query":"query AgentTelemetry { agentTelemetry(limit: 5) { provider model durMs } }"}' "$BEARER"); then
    if printf '%s' "$body" | grep -q '"agentTelemetry"'; then
      ok "agentTelemetry(limit: 5) reachable"
    else
      err "agentTelemetry response missing field: $body"
    fi
  else
    err "agentTelemetry request failed"
  fi
else
  warn "SMOKE_BEARER unset — skipping agentTelemetry (auth-gated)"
fi

# ----------------------------------------------------------------------------
# Section 4 — sample mutation (idempotent)
# ----------------------------------------------------------------------------
section "4. sample mutation"

mutation_body='{"query":"mutation { verifyAudit { ok chainValid } }"}'
if [ -n "$BEARER" ]; then
  body=$(gql_post "$mutation_body" "$BEARER" || true)
else
  body=$(gql_post "$mutation_body" || true)
fi
if [ -n "$body" ]; then
  chain=$(json_extract "$body" '.data.verifyAudit.chainValid' 2>/dev/null || echo "")
  case "$chain" in
    true)
      ok "verifyAudit.chainValid = true"
      ;;
    false)
      warn "verifyAudit.chainValid = false — audit chain is broken (P1 alert, not a smoke fail)" 1>&2
      ok "verifyAudit reachable (chain reports broken — investigate)"
      ;;
    *)
      err "verifyAudit unexpected response: $body"
      ;;
  esac
else
  err "verifyAudit mutation request failed"
fi

# ----------------------------------------------------------------------------
# Section 5 — subscription smoke
# ----------------------------------------------------------------------------
section "5. subscription smoke"

# Translate API URL to ws/wss for the subscription endpoint.
ws_url=$(printf '%s' "$API" | sed -e 's#^http://#ws://#' -e 's#^https://#wss://#')
ws_url="${ws_url}/graphql"

run_ws_smoke_python() {
  python3 - "$ws_url" "$BEARER" <<'PY'
import json, sys, asyncio
try:
    import websockets
except Exception as e:
    print(f"PY_NO_WEBSOCKETS: {e}", file=sys.stderr)
    sys.exit(2)
url = sys.argv[1]
bearer = sys.argv[2] if len(sys.argv) > 2 else ""

async def main():
    async with websockets.connect(url, subprotocols=["graphql-transport-ws"], open_timeout=5) as ws:
        init = {"type": "connection_init", "payload": {}}
        if bearer:
            init["payload"]["authorization"] = f"Bearer {bearer}"
        await ws.send(json.dumps(init))
        ack = await asyncio.wait_for(ws.recv(), timeout=5)
        msg = json.loads(ack)
        if msg.get("type") != "connection_ack":
            print(f"NO_ACK: {ack}", file=sys.stderr); sys.exit(3)
        sub = {"id": "1", "type": "subscribe",
               "payload": {"query": "subscription { costStream { ts usdCents model } }"}}
        await ws.send(json.dumps(sub))
        try:
            await asyncio.wait_for(ws.recv(), timeout=3)
        except asyncio.TimeoutError:
            pass  # no events in 3s is fine
        await ws.close()
        print("ACK_OK")

asyncio.run(main())
PY
}

if have wscat; then
  warn "wscat detected — interactive only; using python3 fallback"
fi

if have websocat; then
  if printf '%s\n' \
       '{"type":"connection_init","payload":{"authorization":"Bearer '"$BEARER"'"}}' \
       '{"id":"1","type":"subscribe","payload":{"query":"subscription { costStream { ts usdCents model } }"}}' \
     | websocat --protocol graphql-transport-ws -n1 -E -t "$ws_url" 2>/dev/null | head -3 \
     | grep -q 'connection_ack'; then
    ok "websocat: connection_ack received from $ws_url"
  else
    err "websocat: did not receive connection_ack from $ws_url"
  fi
elif have python3; then
  if out=$(run_ws_smoke_python 2>&1); then
    if printf '%s' "$out" | grep -q 'ACK_OK'; then
      ok "python3 websockets: connection_ack received from $ws_url"
    else
      err "python3 websockets: unexpected output: $out"
    fi
  else
    # Distinguish "no module" from real failure.
    if printf '%s' "$out" | grep -q 'PY_NO_WEBSOCKETS'; then
      warn "python3 websockets module missing — install with: pip3 install websockets"
      warn "skipping subscription smoke"
    else
      err "python3 websockets smoke failed: $out"
    fi
  fi
else
  warn "neither websocat nor python3 found — skipping subscription smoke"
fi

# ----------------------------------------------------------------------------
# Section 6 — REST deprecation banner
# ----------------------------------------------------------------------------
section "6. REST deprecation banner"

# /projects is a deprecated REST route wrapped in the deprecation middleware.
# We don't care about the body — only the response headers.
hdr_args=(-fsS -D - -o /dev/null)
if [ -n "$BEARER" ]; then
  hdr_args+=(-H "authorization: Bearer $BEARER")
fi
if headers=$(curl "${hdr_args[@]}" "$API/projects" 2>&1); then
  if printf '%s' "$headers" | grep -qiE '^Deprecation:[[:space:]]*true'; then
    ok "Deprecation: true header present on /projects"
    sunset=$(printf '%s' "$headers" | grep -iE '^Sunset:' || true)
    if [ -n "$sunset" ]; then
      ok "$(printf '%s' "$sunset" | tr -d '\r')"
    else
      warn "Sunset header missing on /projects"
    fi
  else
    err "Deprecation header missing on /projects — middleware not wired?"
  fi
else
  # /projects may require auth; an auth failure is fine if the headers still come back.
  if printf '%s' "$headers" | grep -qiE '^Deprecation:[[:space:]]*true'; then
    ok "Deprecation: true header present on /projects (auth-rejected, headers still stamped)"
  else
    err "could not read /projects headers: $headers"
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
