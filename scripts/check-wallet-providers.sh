#!/usr/bin/env bash
# scripts/check-wallet-providers.sh
#
# Verifies that the wallet topper is reachable in prod end-to-end:
# - walletAvailableProviders returns at least one enabled provider
# - walletCreateTopUp(amountUSD: 1, provider: <each>) returns a real
#   checkout URL (Stripe cs_…/Paddle txn_…), not the NOT_CONFIGURED
#   error.
#
# Speaks APQ (production runs GRAPHQL_APQ_LOCKED + open-registration) so
# the requests survive the persisted-query gate.
#
# Usage:
#   bash scripts/check-wallet-providers.sh [API_URL]
#     API_URL defaults to https://api.ironflyer.ai.
#
# Exit code: 0 if at least one provider is enabled AND returns a real
# checkout URL; non-zero otherwise.

set -euo pipefail

API="${1:-${IRONFLYER_API_URL:-https://api.ironflyer.ai}}"
green=$(printf '\033[32m'); red=$(printf '\033[31m'); yellow=$(printf '\033[33m'); reset=$(printf '\033[0m')
ok()   { printf "  %s[OK]%s   %s\n" "$green"  "$reset" "$1"; }
warn() { printf "  %s[WARN]%s %s\n" "$yellow" "$reset" "$1"; }
err()  { printf "  %s[FAIL]%s %s\n" "$red"    "$reset" "$1"; }

command -v jq >/dev/null    || { err "jq is required"; exit 2; }
command -v openssl >/dev/null || { err "openssl is required"; exit 2; }

sha256_hex() { printf '%s' "$1" | openssl dgst -sha256 -hex | awk '{print $NF}'; }

apq_post() {
  local body="$1" bearer="${2:-}" q hash
  q=$(printf '%s' "$body" | jq -r '.query // empty')
  hash=$(sha256_hex "$q")
  body=$(printf '%s' "$body" | jq -c --arg h "$hash" '.extensions = ((.extensions // {}) + {persistedQuery: {version: 1, sha256Hash: $h}})')
  if [ -n "$bearer" ]; then
    curl -sS -X POST "$API/graphql" -H 'content-type: application/json' -H "authorization: Bearer $bearer" --data "$body"
  else
    curl -sS -X POST "$API/graphql" -H 'content-type: application/json' --data "$body"
  fi
}

echo
echo "Wallet topper provider check  ($API)"
echo

# 1. Sign up a throwaway user so we have a JWT.
TS=$(date +%s); RAND=$RANDOM
EMAIL="wallet-probe+${TS}-${RAND}@ironflyer.local"
PASSWORD="wp-${TS}-${RAND}"
SU_QUERY='mutation($email: String!, $password: String!) { signUp(input: {email: $email, password: $password, name: "Wallet Probe"}) { token } }'
SU_BODY=$(jq -n --arg q "$SU_QUERY" --arg e "$EMAIL" --arg p "$PASSWORD" '{query: $q, variables: {email: $e, password: $p}}')
SU_RESP=$(apq_post "$SU_BODY")
TOKEN=$(printf '%s' "$SU_RESP" | jq -r '.data.signUp.token // ""')
if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
  err "signUp failed — cannot continue: $SU_RESP"
  exit 1
fi
ok "signUp($EMAIL) → token acquired"

# 2. walletAvailableProviders.
PROVIDERS_RESP=$(apq_post '{"query":"query { walletAvailableProviders { name label isPrimary } }"}' "$TOKEN")
PROVIDERS=$(printf '%s' "$PROVIDERS_RESP" | jq -c '.data.walletAvailableProviders // empty')
ERRS=$(printf '%s' "$PROVIDERS_RESP" | jq -c '.errors // empty')
if [ -n "$ERRS" ] && [ "$ERRS" != "null" ]; then
  err "walletAvailableProviders errored: $ERRS"
  exit 1
fi
N=$(printf '%s' "$PROVIDERS" | jq -r 'length')
if [ "${N:-0}" -lt 1 ]; then
  err "walletAvailableProviders returned 0 providers — neither Stripe nor Paddle is enabled. Check IRONFLYER_WALLET_PRIMARY_PROVIDER + PADDLE_API_KEY/STRIPE_SECRET_KEY in the orchestrator env."
  exit 1
fi
NAMES=$(printf '%s' "$PROVIDERS" | jq -r '[.[] | "\(.name)\(if .isPrimary then "*" else "" end)"] | join(", ")')
ok "walletAvailableProviders → $N provider(s): $NAMES  (* = primary)"

# 3. walletCreateTopUp for each enabled provider.
FAIL=0
for PROV in $(printf '%s' "$PROVIDERS" | jq -r '.[].name'); do
  TU_QUERY='mutation($a: Float!, $p: String) { walletCreateTopUp(amountUSD: $a, provider: $p) { url sessionID } }'
  TU_BODY=$(jq -n --arg q "$TU_QUERY" --argjson a 1.0 --arg p "$PROV" '{query: $q, variables: {a: $a, p: $p}}')
  TU_RESP=$(apq_post "$TU_BODY" "$TOKEN")
  URL=$(printf '%s' "$TU_RESP" | jq -r '.data.walletCreateTopUp.url // ""')
  SID=$(printf '%s' "$TU_RESP" | jq -r '.data.walletCreateTopUp.sessionID // ""')
  ER=$(printf '%s' "$TU_RESP" | jq -c '.errors // empty')
  if [ -n "$ER" ] && [ "$ER" != "null" ]; then
    err "walletCreateTopUp(provider=$PROV) errored: $ER"
    FAIL=1; continue
  fi
  if [ -z "$URL" ] || [ "$URL" = "null" ]; then
    err "walletCreateTopUp(provider=$PROV) returned no URL: $TU_RESP"
    FAIL=1; continue
  fi
  case "$URL" in
    https://checkout.stripe.com/*|https://*.stripe.com/*) host="Stripe Checkout" ;;
    https://*.paddle.com/*|https://*.paddle.io/*|https://buy.paddle.com/*) host="Paddle Checkout" ;;
    *) host="checkout (unknown vendor)" ;;
  esac
  ok "walletCreateTopUp(provider=$PROV) → $host (session=${SID:0:24}…)"
done

echo
if [ $FAIL -eq 0 ]; then
  printf "%swallet-topper: PASS%s\n" "$green" "$reset"
  exit 0
else
  printf "%swallet-topper: FAIL%s\n" "$red" "$reset"
  exit 1
fi
