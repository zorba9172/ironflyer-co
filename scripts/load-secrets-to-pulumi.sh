#!/usr/bin/env bash
# =============================================================================
# load-secrets-to-pulumi.sh — push every filled secret from .env.production.local
# into the named Pulumi stack as a --secret config value.
#
# Usage:
#   bash scripts/load-secrets-to-pulumi.sh prod-ams3
#
# Requirements:
#   - Pulumi CLI logged in (`pulumi login`)
#   - infra/pulumi-do/ as the working dir (handled automatically)
#   - .env.production.local filled in at repo root
#
# Behaviour:
#   - Empty values are skipped (the orchestrator gracefully degrades).
#   - Already-set Pulumi values are overwritten without confirmation.
#   - Every value is set with --secret (encrypted in stack state).
# =============================================================================

set -euo pipefail

STACK="${1:-}"
if [[ -z "$STACK" ]]; then
    echo "usage: $0 <stack-name>"
    echo "example: $0 prod-ams3"
    exit 1
fi

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ENV_FILE="$REPO_ROOT/.env.production.local"
PULUMI_DIR="$REPO_ROOT/infra/pulumi-do"

if [[ ! -f "$ENV_FILE" ]]; then
    echo "ERROR: $ENV_FILE not found. Copy .env.example and fill it in first."
    exit 1
fi

cd "$PULUMI_DIR"
pulumi stack select "$STACK"

# Map ENV_VAR_NAME → pulumi:config:key
# Format: "ENV_NAME:pulumi:config:key"
declare -a MAPPING=(
    # §1 core
    "JWT_SECRET:ironflyer:jwtSecret"
    "IRONFLYER_SUPERUSER_EMAIL:ironflyer:superuserEmail"
    "IRONFLYER_SUPERUSER_PASSWORD:ironflyer:superuserPassword"

    # §2 paddle
    "PADDLE_API_KEY:ironflyer:paddleApiKey"
    "PADDLE_WEBHOOK_SECRET:ironflyer:paddleWebhookSecret"
    "PADDLE_ENV:ironflyer:paddleEnv"
    "PADDLE_PRICE_PRO:ironflyer:paddlePricePro"
    "PADDLE_PRICE_TEAM:ironflyer:paddlePriceTeam"
    "PADDLE_PRICE_ENTERPRISE:ironflyer:paddlePriceEnterprise"

    # §3 lemonsqueezy
    "LEMONSQUEEZY_API_KEY:ironflyer:lemonsqueezyApiKey"
    "LEMONSQUEEZY_STORE_ID:ironflyer:lemonsqueezyStoreId"
    "LEMONSQUEEZY_WEBHOOK_SECRET:ironflyer:lemonsqueezyWebhookSecret"
    "LEMONSQUEEZY_VARIANT_PRO:ironflyer:lemonsqueezyVariantPro"
    "LEMONSQUEEZY_VARIANT_TEAM:ironflyer:lemonsqueezyVariantTeam"
    "LEMONSQUEEZY_VARIANT_ENTERPRISE:ironflyer:lemonsqueezyVariantEnterprise"

    # §4 stripe
    "STRIPE_SECRET_KEY:ironflyer:stripeSecretKey"
    "STRIPE_WEBHOOK_SECRET:ironflyer:stripeWebhookSecret"
    "STRIPE_PRICE_PRO:ironflyer:stripePricePro"
    "STRIPE_PRICE_TEAM:ironflyer:stripePriceTeam"
    "STRIPE_PRICE_ENTERPRISE:ironflyer:stripePriceEnterprise"
    "STRIPE_METERED_PRICE_PRO:ironflyer:stripeMeteredPricePro"
    "STRIPE_METERED_PRICE_TEAM:ironflyer:stripeMeteredPriceTeam"

    # §4 ai providers
    "ANTHROPIC_API_KEY:ironflyer:anthropicApiKey"
    "OPENAI_API_KEY:ironflyer:openaiApiKey"
    "GEMINI_API_KEY:ironflyer:geminiApiKey"
    "HUGGINGFACE_API_KEY:ironflyer:hfApiKey"
    "DEEPSEEK_API_KEY:ironflyer:deepseekApiKey"
    "VERCEL_AI_GATEWAY_TOKEN:ironflyer:vercelAiGatewayToken"

    # §5 observability
    "SENTRY_DSN_ORCHESTRATOR:ironflyer:sentryDsnOrchestrator"
    "SENTRY_DSN_WEB:ironflyer:sentryDsnWeb"
    "NEXT_PUBLIC_SENTRY_DSN:ironflyer:nextPublicSentryDsn"
    "SENTRY_ORG:ironflyer:sentryOrg"
    "SENTRY_PROJECT:ironflyer:sentryProject"
    "SENTRY_AUTH_TOKEN:ironflyer:sentryAuthToken"
    "DATADOG_API_KEY:ironflyer:datadogApiKey"

    # §6 github
    "GITHUB_CLIENT_ID:ironflyer:githubClientID"
    "GITHUB_CLIENT_SECRET:ironflyer:githubClientSecret"
    "GITHUB_APP_WEBHOOK_SECRET:ironflyer:githubAppWebhookSecret"

    # §7 email
    "RESEND_API_KEY:ironflyer:resendApiKey"
    "RESEND_FROM_EMAIL:ironflyer:resendFromEmail"

    # §8 vercel
    "VERCEL_API_TOKEN:ironflyer:vercelApiToken"

    # §9 cloud
    "DIGITALOCEAN_TOKEN:ironflyer:digitalOceanToken"
    "CLOUDFLARE_API_TOKEN:ironflyer:cloudflareApiToken"
    "CLOUDFLARE_ZONE_ID:ironflyer:cloudflareZoneId"
)

# Load .env.production.local into the current shell.
set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

set_count=0
skip_count=0

for entry in "${MAPPING[@]}"; do
    env_name="${entry%%:*}"
    pulumi_key="${entry#*:}"
    value="${!env_name:-}"

    if [[ -z "$value" ]]; then
        skip_count=$((skip_count + 1))
        continue
    fi

    pulumi config set --secret "$pulumi_key" "$value" > /dev/null
    echo "✓ $pulumi_key"
    set_count=$((set_count + 1))
done

# GitHub App private key — special case: it's a file path, we read the file
if [[ -n "${GITHUB_APP_PRIVATE_KEY_PATH:-}" && -f "$GITHUB_APP_PRIVATE_KEY_PATH" ]]; then
    pulumi config set --secret ironflyer:githubAppPrivateKey "$(cat "$GITHUB_APP_PRIVATE_KEY_PATH")" > /dev/null
    echo "✓ ironflyer:githubAppPrivateKey (read from $GITHUB_APP_PRIVATE_KEY_PATH)"
    set_count=$((set_count + 1))
fi

echo ""
echo "Done. $set_count secrets set, $skip_count empty / skipped."
echo "Run 'pulumi preview' to see what changes."
