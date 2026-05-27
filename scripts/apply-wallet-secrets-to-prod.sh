#!/usr/bin/env bash
# scripts/apply-wallet-secrets-to-prod.sh
#
# Renders the wallet-topper block that needs to land in
# `infra/compose/.env.prod` on the production host (Hetzner AX102).
# Reads the keys from `.env.production.local` at the repo root and
# prompts for the values that aren't there yet (PADDLE_WEBHOOK_SECRET,
# STRIPE_*, IRONFLYER_WALLET_PRIMARY_PROVIDER).
#
# The script only WRITES the block to stdout (and optionally to a file
# you pass with `-o`). It never SSHes anywhere or mutates remote state
# — copy the block manually, or scp the output file to the host.
#
# Usage:
#   bash scripts/apply-wallet-secrets-to-prod.sh                # print to stdout
#   bash scripts/apply-wallet-secrets-to-prod.sh -o /tmp/topper.env
#   bash scripts/apply-wallet-secrets-to-prod.sh --paddle-only  # skip Stripe prompts
#   bash scripts/apply-wallet-secrets-to-prod.sh --stripe-only  # skip Paddle prompts

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ENV_LOCAL="${REPO_ROOT}/.env.production.local"

paddle=true
stripe=true
out=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --paddle-only) stripe=false; shift ;;
    --stripe-only) paddle=false; shift ;;
    -o) out="$2"; shift 2 ;;
    -h|--help)
      sed -n '1,/^set -euo/p' "$0" | sed -n '2,/^# Usage/p; /^# Usage/,/^$/p'
      exit 0
      ;;
    *) echo "unknown flag: $1" >&2; exit 2 ;;
  esac
done

if [ ! -f "$ENV_LOCAL" ]; then
  echo "FATAL: $ENV_LOCAL not found. This script reads PADDLE_API_KEY etc." >&2
  echo "       from there. Either restore the file or supply the values" >&2
  echo "       manually via env vars." >&2
  exit 1
fi

# Read a key from .env.production.local. Quoted, comment-free lines only.
read_env_local() {
  local key="$1"
  awk -F= -v k="$key" '$1==k {sub(/^[^=]*=/,""); print}' "$ENV_LOCAL"
}

# Prompt for a value if the caller didn't pass it via env.
prompt() {
  local key="$1" label="$2" cur="${!1:-}"
  if [ -n "$cur" ]; then printf '%s' "$cur"; return; fi
  printf '%s: ' "$label" >&2
  local v
  IFS= read -r v
  printf '%s' "$v"
}

PADDLE_API_KEY=$(read_env_local PADDLE_API_KEY)
PADDLE_ENV=$(read_env_local PADDLE_ENV)
PADDLE_ENV=${PADDLE_ENV:-live}

block=""
emit() { block+="$1"$'\n'; }

emit ""
emit "# --- Wallet topper (added by scripts/apply-wallet-secrets-to-prod.sh) -------"

if $paddle; then
  if [ -z "$PADDLE_API_KEY" ]; then
    echo "FATAL: PADDLE_API_KEY missing from $ENV_LOCAL." >&2
    exit 1
  fi
  PADDLE_WEBHOOK_SECRET_VAL=$(prompt PADDLE_WEBHOOK_SECRET "PADDLE_WEBHOOK_SECRET (from Paddle dashboard → Notifications → secret)")
  if [ -z "$PADDLE_WEBHOOK_SECRET_VAL" ]; then
    echo "FATAL: PADDLE_WEBHOOK_SECRET is required (paste from Paddle dashboard)." >&2
    exit 1
  fi
  emit "PADDLE_API_KEY=${PADDLE_API_KEY}"
  emit "PADDLE_WEBHOOK_SECRET=${PADDLE_WEBHOOK_SECRET_VAL}"
  emit "PADDLE_ENV=${PADDLE_ENV}"
  emit "PADDLE_WALLET_SUCCESS_URL=https://app.ironflyer.ai/wallet/topup"
  emit "PADDLE_WALLET_CANCEL_URL=https://app.ironflyer.ai/wallet/topup?cancelled=1"
fi

if $stripe; then
  STRIPE_SECRET_KEY_VAL=$(prompt STRIPE_SECRET_KEY "STRIPE_SECRET_KEY (rk_live_…)")
  STRIPE_WEBHOOK_SECRET_VAL=$(prompt STRIPE_WEBHOOK_SECRET "STRIPE_WEBHOOK_SECRET (whsec_…)")
  if [ -n "$STRIPE_SECRET_KEY_VAL" ]; then
    if [ -z "$STRIPE_WEBHOOK_SECRET_VAL" ]; then
      echo "FATAL: STRIPE_WEBHOOK_SECRET required when STRIPE_SECRET_KEY is set." >&2
      exit 1
    fi
    emit "STRIPE_SECRET_KEY=${STRIPE_SECRET_KEY_VAL}"
    emit "STRIPE_WEBHOOK_SECRET=${STRIPE_WEBHOOK_SECRET_VAL}"
    emit 'STRIPE_SUCCESS_URL=https://app.ironflyer.ai/wallet/topup?session_id={CHECKOUT_SESSION_ID}'
    emit "STRIPE_CANCEL_URL=https://app.ironflyer.ai/wallet/topup?cancelled=1"
  else
    echo "(skipping Stripe — verification probably still in progress)" >&2
  fi
fi

primary_default=stripe
if $paddle && ! $stripe; then primary_default=paddle; fi
PRIMARY=$(prompt IRONFLYER_WALLET_PRIMARY_PROVIDER "IRONFLYER_WALLET_PRIMARY_PROVIDER [stripe|paddle, default ${primary_default}]")
PRIMARY=${PRIMARY:-$primary_default}
case "$PRIMARY" in
  stripe|paddle) ;;
  *) echo "FATAL: primary must be 'stripe' or 'paddle' (got '$PRIMARY')." >&2; exit 1 ;;
esac
emit "IRONFLYER_WALLET_PRIMARY_PROVIDER=${PRIMARY}"
emit "# --- end wallet topper ------------------------------------------------------"

if [ -n "$out" ]; then
  printf '%s' "$block" > "$out"
  chmod 600 "$out"
  echo "wrote $(wc -l < "$out" | tr -d ' ') lines to $out (chmod 600)" >&2
  echo "next: scp '$out' ironflyer@<AX102>:/path/to/ironflyer/infra/compose/.env.prod-topper.fragment" >&2
  echo "then on the host: cat .env.prod-topper.fragment >> .env.prod && chmod 600 .env.prod && rm .env.prod-topper.fragment" >&2
else
  printf '%s' "$block"
fi
