# Ironflyer launch checklist

This is the operator runbook for taking Ironflyer from "runs locally"
to "first paying customer can sign up and run a billable execution."
Every section ends with the **decision Moshe must make** + the
**credential to provide** + the **commands I will run with you in the
room**.

Estimated time start-to-finish, all credentials ready: **30–45
minutes**. Without credentials: blocked.

---

## 0. Pre-flight (do this first, takes 10 minutes)

- [ ] Pick a primary domain. Suggested: `ironflyer.app` or `ironflyer.dev`.
  Both `.app` and `.dev` come with HSTS baked in by Google → free TLS
  hardening. Avoid `.com` for an AI product launching in 2026 (price +
  squatting + brand pollution).
- [ ] Confirm you have a credit card available for: Vercel ($20/mo Pro
  if going over the hobby tier), Stripe (free), GHCR (free, hosted with
  GitHub), Anthropic (~$50 prepaid credit to start).
- [ ] Decide pricing tiers. See `docs/OVERNIGHT_2026-05-28.md` for the
  research-backed recommendation:
  - Free $0/mo — $5 wallet credit on signup, no compliance gates.
  - Pro $79/mo — full AppSec gates, full Studio, deploys go to user's
    own Vercel / Cloudflare account.
  - Team $399/mo — SOC2 + HIPAA gates, audit log, 5 seats included.
  - Enterprise — custom, starts $2k/mo.

---

## 1. Hosting — Vercel

**Why Vercel:** the web client is Next.js 15 and the home page already
mentions one-click Vercel deploy as the artifact every paid execution
produces. Same vendor for our own marketing site keeps the story
coherent.

- [ ] Create a Vercel account (Moshe).
- [ ] Create a new project from the `clients/web` directory.
- [ ] In Vercel project settings → Domains, add `ironflyer.app` and
  `www.ironflyer.app`. Vercel issues TLS automatically.
- [ ] Generate a Vercel API token at https://vercel.com/account/tokens
  with **Full Access** scope, project-scoped to this project only.
  **Save the token** — we'll use it for CLI deploys.

**Credential needed:** `VERCEL_TOKEN`, `VERCEL_PROJECT_ID`.

---

## 2. Database — Postgres (Neon recommended)

**Why Neon:** serverless Postgres with branching, $0 free tier, the
orchestrator already supports `IRONFLYER_DB_DRIVER=postgres`. Migrations
in `core/orchestrator/migrations/` apply on boot.

- [ ] Create a Neon project (`https://neon.tech`).
- [ ] Create a database named `ironflyer`.
- [ ] Copy the connection string. Pooled URL goes to the app; direct
  URL goes to the migration runner.

**Credential needed:** `POSTGRES_URL` (pooled), `POSTGRES_MIGRATIONS_URL`
(direct).

---

## 3. AI provider — Anthropic

**Why Anthropic first:** the locked default per `CLAUDE.md`. Claude
Sonnet 4.6 for general work, Opus 4.7 for reasoning, Haiku 4.5 for
inline completions.

- [ ] Create an Anthropic API key at https://console.anthropic.com.
- [ ] Top up with at least $50 prepaid credits (covers ~5,000 Sonnet
  calls or ~200 Opus reasoning calls — enough for a soft launch).
- [ ] (Optional but recommended) Repeat for OpenAI + Gemini so
  ProfitGuard has cheaper fallback paths.

**Credential needed:** `ANTHROPIC_API_KEY` (required), `OPENAI_API_KEY`
(optional), `GEMINI_API_KEY` (optional).

---

## 4. Payments — Stripe

**Critical:** without this, wallet top-ups can't credit the ledger.
Without wallet, every execution returns 402 (law 1). No revenue.

- [ ] Activate the Stripe account for **live mode**. Moshe will need to
  submit business verification (passport, business address, bank
  details). Allow 1–3 days if not done yet.
- [ ] In Stripe Dashboard → Products, create one product called
  "Ironflyer Wallet Credits" with prices for each top-up amount:
  - $10, $50, $200, $500. (Top-up sizes match the pricing tiers.)
- [ ] Save the **price ID** for each.
- [ ] Stripe Dashboard → Developers → API keys → copy the **live**
  publishable + secret key.
- [ ] Stripe Dashboard → Developers → Webhooks → Add endpoint.
  - URL: `https://api.ironflyer.app/budget/webhook`
  - Events: `checkout.session.completed`, `payment_intent.succeeded`.
  - Copy the **webhook signing secret**.

**Credentials needed:** `STRIPE_SECRET_KEY` (live), `STRIPE_PUBLISHABLE_KEY`
(live), `STRIPE_WEBHOOK_SECRET`, `STRIPE_PRICE_TOPUP_10`, `..._50`,
`..._200`, `..._500`.

---

## 5. Workspace image — GHCR

**Why this matters:** the Docker runtime driver pulls
`ghcr.io/zorba9172/ironflyer-workspace:latest`. Without this image
published, paid executions can't allocate workspaces, and the AppSec
scanners have nowhere to run.

- [ ] Authenticate `docker login ghcr.io` with a GitHub PAT that has
  `write:packages` scope.
- [ ] Build + push the workspace image:
  ```bash
  cd /Users/moshemoran/workspace/ironflyer-copilot/ironflyer
  TAG=latest ./scripts/build-docker.sh
  docker tag ironflyer/workspace:latest ghcr.io/zorba9172/ironflyer-workspace:latest
  docker push ghcr.io/zorba9172/ironflyer-workspace:latest
  ```
- [ ] Verify the image is public (GHCR defaults to private; change
  visibility in https://github.com/users/zorba9172/packages/container/ironflyer-workspace/settings).

**Credential needed:** GitHub PAT with `write:packages`.

---

## 6. Runtime host

**Why this is the hardest piece:** the runtime spawns per-user Docker
containers. Vercel + Neon are pure SaaS; the runtime needs a Linux box
with Docker socket access.

Options, ordered by simplicity:

**A. Hetzner CCX23 (recommended for launch).** $40/mo, 4 vCPU, 16 GB
RAM, NVMe. Enough for ~30 concurrent workspaces. SSH in, install
Docker, point at `ghcr.io/zorba9172/ironflyer-workspace`, run
`scripts/runtime-host.sh` (will write this when you give green light).

**B. Fly.io machines with Docker passthrough.** Per-machine billing,
scales to zero. ~$0.05/hr per machine. Better economics if traffic is
bursty. Setup is more involved.

**C. Skip per-user runtimes for v1.** Use the **Mock driver** in prod,
serve generated code as static files only (no real terminal / no real
build). This neuters the "real Linux workspace" pillar but launches in
30 minutes flat. Recommended if you want to validate willingness-to-pay
before paying for infra.

**Credential needed:** `RUNTIME_HOST` (the IP / hostname), SSH key, or
Fly.io token, or "mock for v1".

---

## 7. DNS + apex

Once Vercel issues the cert and the runtime host has a public IP:

- [ ] `ironflyer.app` `A` → Vercel anycast IPs (Vercel will show them).
- [ ] `www.ironflyer.app` `CNAME` → `cname.vercel-dns.com`.
- [ ] `api.ironflyer.app` `A` → orchestrator host (or Vercel function
  if orchestrator lives on Fly/Hetzner).
- [ ] `runtime.ironflyer.app` `A` → runtime host (if separate).

---

## 8. Final env

Once everything above is filled in, the production `.env.production`
looks like this. **Do not commit this file.**

```bash
# Hosting
NEXT_PUBLIC_IRONFLYER_API_URL=https://api.ironflyer.app
NEXT_PUBLIC_SITE_URL=https://ironflyer.app

# Database
POSTGRES_URL=postgres://...neon.tech/ironflyer
IRONFLYER_DB_DRIVER=postgres

# AI
ANTHROPIC_API_KEY=sk-ant-...
OPENAI_API_KEY=sk-...        # optional
GEMINI_API_KEY=...           # optional

# Auth (generate a fresh secret per environment)
IRONFLYER_JWT_SECRET=<openssl rand -hex 48>

# Stripe (live)
STRIPE_SECRET_KEY=sk_live_...
STRIPE_PUBLISHABLE_KEY=pk_live_...
STRIPE_WEBHOOK_SECRET=whsec_...
STRIPE_PRICE_TOPUP_10=price_...
STRIPE_PRICE_TOPUP_50=price_...
STRIPE_PRICE_TOPUP_200=price_...
STRIPE_PRICE_TOPUP_500=price_...

# Runtime
IRONFLYER_RUNTIME_DRIVER=docker
IRONFLYER_RUNTIME_DOCKER_IMAGE=ghcr.io/zorba9172/ironflyer-workspace:latest
IRONFLYER_RUNTIME_ADDR=:8090
RUNTIME_EFS_MOUNT=/var/lib/ironflyer/workspaces

# Observability (optional but recommended for paid customers)
SENTRY_DSN=
```

---

## 9. Smoke test

After deploy, run `scripts/smoke.sh` from your laptop against the
production URL. It exercises:

- `/healthz`, `/readyz`, `/version`
- POST a tiny describeIdea → expects 402 Payment Required (no wallet
  funds yet)
- Stripe webhook delivery → wallet credit appears
- Re-run describeIdea → 200 + execution starts
- AppSec gate runs → expect at least one finding on a deliberately
  insecure prompt

If all green: announce.

---

## 10. What I will not do without you in the room

- Push to a remote git host (`git push`).
- `docker push` to GHCR.
- Run `stripe listen` against live mode.
- `vercel --prod` deploy.
- Update DNS.
- Take down `--profile full` services on the dev box.

Every one of those is shared-state or paid-money. Wake me up.
