#!/usr/bin/env bash
# Ironflyer end-to-end demo — "Stridon Click" landing page.
#
# Reproducible transcript of the wow-loop flow Closure Agent L
# captured on 2026-05-26. The script:
#
#   1. signs up a random demo user against the local orchestrator
#      (relies on the dev wallet seed IRONFLYER_DEV_WALLET_SEED_USD=50,
#      so no Stripe top-up is required),
#   2. invokes the V22 entrypoint `mutation describeIdea` with the
#      Stridon Click prompt — describeIdea creates the project, picks
#      the blueprint, places the wallet hold (law 1), admits + starts
#      the execution in one round trip,
#   3. subscribes to `subscription executionFeed(id)` over the
#      graphql-transport-ws protocol and captures up to 60 events
#      (or whatever lands inside 90 seconds),
#   4. stops the execution after the 90 s capture window (the demo
#      contract is bounded — `mutation stopExecution`) and pulls
#      the 6-artifact `query executionSupportBundle` plus the
#      final `query execution`, `query wallet`, and
#      `query profitGuardDecisions` rows,
#   5. prints the full JSON of every step to stdout so the run can
#      be diff-ed against docs/DEMO_RUN.md.
#
# GraphQL-only: no REST endpoints are touched.
#
# Requirements: curl, jq, and either python3+websockets or websocat
# for the subscription leg. Without a websocket client the script
# still completes the rest of the flow and prints a [WARN].
#
# Usage:
#   IRONFLYER_API_URL=http://localhost:8080 ./scripts/demo/run-stridon-click.sh

set -euo pipefail

API="${IRONFLYER_API_URL:-http://localhost:8080}"
EVENTS_CAP="${DEMO_EVENTS:-60}"
WS_TIMEOUT="${DEMO_WS_TIMEOUT:-90}"

IDEA_TEXT="Build a marketing landing page for a fictional ergonomic mechanical keyboard brand called 'Stridon Click', highlighting the ortholinear layout, recycled-aluminum chassis, and Bluetooth+USB-C hybrid wireless"

bold=$(printf '\033[1m'); reset=$(printf '\033[0m')
section() { printf "\n%s== %s ==%s\n" "$bold" "$1" "$reset"; }
require() { command -v "$1" >/dev/null 2>&1 || { echo "missing dependency: $1" >&2; exit 2; }; }

require curl
require jq

gql() {
  local body="$1" bearer="${2:-}"
  if [ -n "$bearer" ]; then
    curl -sS -X POST "$API/graphql" \
      -H 'content-type: application/json' \
      -H "authorization: Bearer $bearer" \
      --data "$body"
  else
    curl -sS -X POST "$API/graphql" \
      -H 'content-type: application/json' \
      --data "$body"
  fi
}

# ---------------------------------------------------------------- 1. sign up
section "1. signUp"
TS=$(date +%s); RAND=$RANDOM
EMAIL="demo+${TS}-${RAND}@ironflyer.local"
PASSWORD="demo-${TS}-${RAND}"

SIGNUP_QUERY='mutation SignUp($email: String!, $password: String!) { signUp(input: {email: $email, password: $password, name: "Stridon Demo"}) { token user { id email plan } } }'
SIGNUP_BODY=$(jq -n --arg q "$SIGNUP_QUERY" --arg e "$EMAIL" --arg p "$PASSWORD" \
  '{query: $q, variables: {email: $e, password: $p}}')

SIGNUP_RESP=$(gql "$SIGNUP_BODY")
echo "$SIGNUP_RESP" | jq .
TOKEN=$(echo "$SIGNUP_RESP" | jq -r '.data.signUp.token')
USER_ID=$(echo "$SIGNUP_RESP" | jq -r '.data.signUp.user.id')
[ -n "$TOKEN" ] && [ "$TOKEN" != "null" ] || { echo "no token; aborting" >&2; exit 1; }

# ---------------------------------------------------------------- 2. wallet
section "2. wallet (initial)"
gql '{"query":"query { wallet { tenantID balanceUSD holdUSD availableUSD lifetimeTopUpUSD updatedAt } }"}' "$TOKEN" | jq .

# ------------------------------------------------------------- 3. describeIdea
section "3. describeIdea (Stridon Click)"
DESCRIBE_Q='mutation Describe($text: String!) { describeIdea(input: {text: $text}) { project { id name } execution { id status budgetUSD reservedUSD spentUSD blueprintID workspaceID projectID metadata } idea { title summary blueprintID blueprintReason suggestedBudgetUSD stopLossUSD confidence tags } costEstimate { lowUSD medianUSD highUSD p95USD confidence basedOnRuns caveat } } }'
DESCRIBE_BODY=$(jq -n --arg q "$DESCRIBE_Q" --arg text "$IDEA_TEXT" '{query: $q, variables: {text: $text}}')
DESCRIBE_RESP=$(gql "$DESCRIBE_BODY" "$TOKEN")
echo "$DESCRIBE_RESP" | jq .

EXEC_ID=$(echo "$DESCRIBE_RESP" | jq -r '.data.describeIdea.execution.id')
PROJECT_ID=$(echo "$DESCRIBE_RESP" | jq -r '.data.describeIdea.project.id')
[ -n "$EXEC_ID" ] && [ "$EXEC_ID" != "null" ] || { echo "no execution ID; aborting" >&2; exit 1; }

section "4. wallet (after describeIdea — hold should be visible)"
gql '{"query":"query { wallet { balanceUSD holdUSD availableUSD } }"}' "$TOKEN" | jq .

# --------------------------------------------------- 5. executionFeed subscribe
section "5. executionFeed subscription (cap ${EVENTS_CAP} events / ${WS_TIMEOUT}s idle)"
WS_URL=$(printf '%s' "$API" | sed -e 's#^http://#ws://#' -e 's#^https://#wss://#')
WS_URL="${WS_URL}/graphql"

PY_BIN=""
for cand in /tmp/iron_ws_venv/bin/python python3; do
  if command -v "$cand" >/dev/null 2>&1 && "$cand" -c "import websockets" 2>/dev/null; then
    PY_BIN="$cand"; break
  fi
done

if [ -n "$PY_BIN" ]; then
  V22_TOKEN="$TOKEN" V22_EXEC_ID="$EXEC_ID" V22_WS_URL="$WS_URL" \
  V22_EVENTS="$EVENTS_CAP" V22_WS_TIMEOUT="$WS_TIMEOUT" \
  "$PY_BIN" - <<'PY'
import os, json, asyncio, sys
import websockets

url = os.environ["V22_WS_URL"]
token = os.environ["V22_TOKEN"]
exec_id = os.environ["V22_EXEC_ID"]
events_cap = int(os.environ["V22_EVENTS"])
timeout = float(os.environ["V22_WS_TIMEOUT"])

TERMINAL = {"succeeded","failed","stopped","killed"}

async def main():
    async with websockets.connect(url, subprotocols=["graphql-transport-ws"], open_timeout=5) as ws:
        await ws.send(json.dumps({"type":"connection_init","payload":{"authorization":f"Bearer {token}"}}))
        ack = await asyncio.wait_for(ws.recv(), timeout=5)
        print(json.dumps({"meta":"ack","msg":json.loads(ack)}))
        sub = {"id":"1","type":"subscribe","payload":{
            "query":"subscription($id: ID!) { executionFeed(id: $id) { executionID eventType payload createdAt } }",
            "variables":{"id":exec_id}}}
        await ws.send(json.dumps(sub))
        count = 0
        while count < events_cap:
            try:
                raw = await asyncio.wait_for(ws.recv(), timeout=timeout)
            except asyncio.TimeoutError:
                print(json.dumps({"meta":"idle_timeout","received":count}))
                break
            count += 1
            try:
                ev = json.loads(raw)
            except Exception:
                print(json.dumps({"meta":"raw","raw":raw})); continue
            t = ev.get("type")
            if t == "next":
                p = ev.get("payload",{}).get("data",{}).get("executionFeed",{})
                print(json.dumps({"n":count,"eventType":p.get("eventType"),"createdAt":p.get("createdAt"),"payload":p.get("payload")}))
                if p.get("eventType","") in TERMINAL: break
            elif t in ("complete","error"):
                print(json.dumps({"meta":t,"payload":ev.get("payload")})); break
            else:
                print(json.dumps({"meta":"other","msg":ev}))
        try: await ws.send(json.dumps({"id":"1","type":"complete"}))
        except Exception: pass
asyncio.run(main())
PY
else
  echo "[WARN] no websockets client (install: python3 -m venv /tmp/iron_ws_venv && /tmp/iron_ws_venv/bin/pip install websockets); skipping subscription"
fi

# ----------------------------------------------------- 6. stop if still running
section "6. stopExecution (demo timeout — bounded capture window)"
STATUS=$(gql "{\"query\":\"query { execution(id:\\\"$EXEC_ID\\\") { status } }\"}" "$TOKEN" | jq -r '.data.execution.status')
echo "current status: $STATUS"
if [ "$STATUS" = "running" ] || [ "$STATUS" = "admitted" ] || [ "$STATUS" = "created" ]; then
  gql "{\"query\":\"mutation { stopExecution(id:\\\"$EXEC_ID\\\", reason:\\\"demo timeout\\\") { id status spentUSD reservedUSD refundedUSD endedAt failureReason } }\"}" "$TOKEN" | jq .
fi

# ----------------------------------------------------- 7. wow-loop artifacts
section "7. executionSupportBundle (the 6 wow-loop artifacts)"
gql "{\"query\":\"query { executionSupportBundle(executionID:\\\"$EXEC_ID\\\") { executionID tenantID status previewURL productionURL changedFiles patchCount gateReport { completionScore stages { name status issuesCount } } securityReport { passRate blockedDeploy findings { severity ruleID path line summary } } costReport { revenueUSD providerCostUSD sandboxCostUSD storageCostUSD deploymentCostUSD grossMarginPct } nextBestAction { kind title reason cta } generatedAt } }\"}" "$TOKEN" | jq .

section "8. execution (final row)"
gql "{\"query\":\"query { execution(id:\\\"$EXEC_ID\\\") { id status budgetUSD reservedUSD spentUSD refundedUSD revenueUSD providerCostUSD sandboxCostUSD storageCostUSD deploymentCostUSD completionScore grossMarginPct workspaceID projectID failureReason createdAt admittedAt startedAt endedAt } }\"}" "$TOKEN" | jq .

section "9. wallet (final)"
gql '{"query":"query { wallet { balanceUSD holdUSD availableUSD lifetimeTopUpUSD lifetimeSpendUSD } }"}' "$TOKEN" | jq .

section "10. profitGuardDecisions"
gql "{\"query\":\"query { profitGuardDecisions(executionID:\\\"$EXEC_ID\\\") { id enforcementPoint decision reason spentUSD reservedUSD estimatedStepCostUSD expectedCompletionDelta riskScore createdAt } }\"}" "$TOKEN" | jq .

section "done"
echo "execution_id : $EXEC_ID"
echo "project_id   : $PROJECT_ID"
echo "demo email   : $EMAIL"
