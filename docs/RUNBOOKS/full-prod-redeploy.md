# Full production redeploy — pull every app image at once

Use this when CI on `main` is green and you want the live Hetzner stack
to pick up the newly-built images from GHCR. This is the single step
that closes the gap between "merged + built" and "live in prod."

## Why this is needed

The `Docker images` workflow (and the `Build + push images` jobs inside
`CI`) publish four app images to GHCR on every green push to `main`:

- `ghcr.io/zorba9172/ironflyer-orchestrator:latest`
- `ghcr.io/zorba9172/ironflyer-runtime:latest`
- `ghcr.io/zorba9172/ironflyer-web:latest`
- `ghcr.io/zorba9172/ironflyer-ironflyer-code:latest`

But the AX102 host keeps running whatever it last pulled. Until you
`docker compose pull` + `up -d`, the running containers stay on the old
image — which is why, after a merge:

- `GET /version` keeps returning `dev / unknown` (orchestrator stale).
- New web routes like `/start` and `/vscode` 404 (web stale).
- `walletCreateTopUp` returns `wallet_topper: not configured` even after
  the multi-provider code merged (orchestrator stale).

## The exact service names

`infra/compose/docker-compose.prod.yml` runs the app images as:

| Service | Image | Replicas |
|---|---|---|
| `orchestrator-1`, `orchestrator-2` | `ironflyer-orchestrator` | 2 (Caddy load-balances) |
| `runtime` | `ironflyer-runtime` | 1 |
| `web-1`, `web-2` | `ironflyer-web` | 2 (Caddy load-balances) |

> Note: there is **no** `runtime-1` / `runtime-2` — runtime is a single
> service named `runtime`. (An earlier draft of GAP_CLOSURE_2026-05-27.md
> said `runtime-1 runtime-2`; that was wrong.)

## One-click path (GitHub Actions)

If the prod SSH secrets are configured (`PROD_SSH_HOST`, `PROD_SSH_KEY`,
optionally `PROD_SSH_USER` / `PROD_DEPLOY_DIR` / `PROD_SSH_KNOWN_HOSTS`),
skip the manual SSH and dispatch the deploy workflow:

```bash
gh workflow run deploy-prod.yml \
  -f version=latest \
  -f "services=orchestrator-1 orchestrator-2 runtime web-1 web-2"
# watch it:
gh run watch "$(gh run list --workflow=deploy-prod.yml --limit 1 --json databaseId -q '.[0].databaseId')"
```

The workflow SSHes into the host, pins `IRONFLYER_VERSION` in
`.env.prod`, pulls + rolls the services, and verifies `/version` is no
longer `dev`. It is `workflow_dispatch`-only — prod deploys stay
deliberate. Source: [`.github/workflows/deploy-prod.yml`](../../.github/workflows/deploy-prod.yml).

## Manual path (SSH)

```bash
ssh ironflyer@<AX102-IP>
cd /path/to/ironflyer/infra/compose

# 1. Pin the version you want live. `latest` follows the tip of main;
#    pin to a sha-<short> or a vX.Y.Z tag for a reproducible rollback target.
#    Edit IRONFLYER_VERSION in .env.prod, or export it inline:
export IRONFLYER_VERSION=latest

# 2. Pull the fresh images for all five app containers.
docker compose -f docker-compose.prod.yml --env-file .env.prod pull \
  orchestrator-1 orchestrator-2 runtime web-1 web-2

# 3. Recreate them. Caddy keeps the other replica serving traffic while
#    each one rolls, so orchestrator + web stay up with zero downtime.
docker compose -f docker-compose.prod.yml --env-file .env.prod up -d \
  orchestrator-1 orchestrator-2 runtime web-1 web-2

# 4. Watch them go healthy (~1-3 min).
docker compose -f docker-compose.prod.yml --env-file .env.prod ps \
  orchestrator-1 orchestrator-2 runtime web-1 web-2
```

## Verify from your laptop

```bash
# Orchestrator picked up the new binary — version is no longer dev/unknown:
curl -sS https://api.ironflyer.ai/version
# Expect: {"version":"sha-XXXXXXX" or "vX.Y.Z","commit":"<full-sha>","buildTime":"<ISO>", ...}

# Web picked up the new routes:
curl -sS -o /dev/null -w "%{http_code}\n" https://ironflyer.ai/start    # expect 200
curl -sS -o /dev/null -w "%{http_code}\n" https://ironflyer.ai/vscode   # expect 200

# Full economic + API contract:
IRONFLYER_API_URL=https://api.ironflyer.ai \
  SMOKE_METRICS_TOKEN="$IRONFLYER_METRICS_TOKEN" \
  bash scripts/smoke.sh
# Expect every section green except section 7 (wallet top-up) until the
# Stripe/Paddle config from docs/RUNBOOKS/wallet-topper-finish.md lands.

# Once the wallet topper config is in place:
bash scripts/check-wallet-providers.sh https://api.ironflyer.ai
```

## Rollback

Pin `IRONFLYER_VERSION` to the previous `sha-<short>` (visible in the
GHCR package history or in `docker compose images`), then re-run steps
2-4. Because the images are immutable and content-addressed, rollback is
just pulling the older tag.

## Relationship to the other runbooks

- [`wallet-topper-finish.md`](wallet-topper-finish.md) — the
  Stripe/Paddle dashboard + `.env.prod` steps. Do those **before** this
  redeploy if you want the wallet topper live in the same roll.
- [`upgrade.md`](upgrade.md) — the canonical rolling-upgrade runbook for
  routine version bumps.
- [`rollback.md`](rollback.md) — the canonical rollback runbook.
