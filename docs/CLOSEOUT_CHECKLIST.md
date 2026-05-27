# Ironflyer — Production Closeout Checklist

This checklist consolidates everything an operator needs to take
`ironflyer.ai` from "code-complete" to "live in production on
DigitalOcean". It is intentionally pragmatic: every step is either a
command you run, a secret you set, or a screen you read.

Read it top-to-bottom on the first deploy. On subsequent deploys §2c
(`pulumi up`) is the only required pass; §2b should be a no-op as
long as nothing in `infra/pulumi/Pulumi.prod.yaml` has changed.

Related docs:

- [`DEPLOY.md`](../DEPLOY.md) — the full deploy reference; treat the
  secrets table as the canonical list.
- [`docs/RUNBOOKS/cold-start.md`](RUNBOOKS/cold-start.md) — the
  verified short version of §2c.
- [`docs/PROJECT_CLOSEOUT_PLAN.md`](PROJECT_CLOSEOUT_PLAN.md) — the
  10-item Definition of Done this checklist closes against.
- [`docs/PERF_BUDGETS.md`](PERF_BUDGETS.md) — SLOs to watch in
  production.

---

## §2a — Pre-flight: what's already done

Context for an incoming operator. The recent commit wave (see
`git log feat/vscode-extension`) closed the following major lanes:

- **Monorepo restructure** — `core/` (orchestrator, runtime, cli)
  + `clients/` (web, vscode-extension, scrcpy-bridge). The
  orchestrator internal package split into 5 domains:
  `business/`, `operations/`, `ai/`, `runtime/`, `platform/`.
- **Two payment providers** — Stripe + Paddle behind the
  `PaymentProvider` interface in
  `core/orchestrator/internal/business/payments/`. Provider
  selected at runtime via `IRONFLYER_PAYMENT_PROVIDER`.
- **Super-user bootstrap** — `IRONFLYER_SUPERUSER_EMAIL` +
  `IRONFLYER_SUPERUSER_PASSWORD` seed an admin user at startup
  (idempotent; bcrypt + reset on env change).
- **Sentry / OpenTelemetry / production metrics** — both Go
  modules wired against `IRONFLYER_SENTRY_DSN` and OTLP exporters;
  Prometheus scrape at `/metrics`.
- **Internal `pkg/` shared utils** — bring-up-friendly helpers
  pulled out of `cmd/` (logger, ctx, ids, time, secret redaction).
- **7 perf indexes + DrainRefinements batch** — every hot
  ledger / execution / event read has an index; refinements drain
  in a single Postgres round-trip.
- **Graceful shutdown + 10 daemon supervision** — `cmd/orchestrator/
  main.go` runs 10 background goroutines under a single supervisor
  with SIGTERM fan-out and bounded grace.
- **GraphQL caps** — complexity + depth limits, persisted-query
  (APQ) policy, introspection toggled off in production, safe
  error masking.
- **Web bundle + Apollo cache hardening** — bundle-size budget
  enforced via the BundleSizeGate; Apollo cache TypePolicy
  per-tenant; SSR streaming where it matters.
- **Anti-Bloat Engine MVP** — Capability Atlas + Architecture
  Manifest + 10 lifecycle gates (`reuse_check`, `dep_graph`,
  `arch_boundary`, `dedup`, `deadcode`, `complexity`,
  `bundle_size`, `mem_leak`, `perf_budget`, `vuln_scan`). See
  [`docs/ANTI_BLOAT_ENGINE.md`](ANTI_BLOAT_ENGINE.md).
- **ProfitGuard coverage** — every premium model call, sandbox
  allocation, retry loop, mobile build, deploy, and failover
  decision passes through `profitguard.Decide`. Mobile build gate
  + failover gate added in this wave.
- **ClickHouse async inserts + `raw_*` subscriptions** —
  GraphQL subscriptions stream the raw event log; ClickHouse
  consumer batches inserts asynchronously off the hot path.
- **SurrealDB as production-default memory backend** — selected
  by `IRONFLYER_MEMORY_BACKEND=surreal` (default in prod profiles);
  in-process `memory` ring buffer remains the lean dev default.
- **Cold-start parallelism + lazy modes** — daemons start in
  parallel groups, heavy subsystems (ClickHouse, Temporal) lazy-
  init on first use.
- **Feedback Brain MVP** — OutcomeEvent stream + ClickHouse facts
  + Pattern Miner + Closure Intelligence + Weakness identification.
  Web dashboard at `/cockpit/learning` with 6 panels (LearningPulse,
  BanditConfidence, GateFailureRate, BlueprintSuccess, Weaknesses,
  LearningRecap). Anti-Bloat moat made measurable. See
  [`docs/FEEDBACK_BRAIN.md`](FEEDBACK_BRAIN.md).

If any of the above is unclear, re-read
`docs/PROJECT_CLOSEOUT_PLAN.md` before continuing.

---

## §2b — Operator preflight (do this BEFORE `pulumi up`)

Each step has a verification command. Do not skip the verifications —
they are the cheap way to catch a missing secret before Pulumi spends
20 minutes creating a half-broken stack.

### 1. Domain DNS

Register `ironflyer.ai` (one-time) and point its name servers at
DigitalOcean's:

```text
ns1.digitalocean.com
ns2.digitalocean.com
ns3.digitalocean.com
```

Verify:

```bash
dig +short NS ironflyer.ai
# Expect three ns?.digitalocean.com lines.
```

If `dig` returns the registrar's defaults, the NS update has not
propagated yet — wait 30 min and re-check. Pulumi will create the
domain record + A/AAAA inside DO, but only if the NS chain is correct.

### 2. Pulumi config — required secrets

The full set lives in `DEPLOY.md §4`. Mirror it into the
`infra/pulumi` stack:

```bash
cd infra/pulumi
pulumi stack select prod
pulumi config                       # eyeball the current state
```

Required keys (set each with `pulumi config set --secret`):

| Key | Source |
| --- | --- |
| `ironflyer:jwtSecret` | `openssl rand -hex 32` |
| `ironflyer:anthropicApiKey` | Anthropic console |
| `ironflyer:stripeSecretKey` | Stripe live key (`sk_live_...`) |
| `ironflyer:stripeWebhookSecret` | Stripe webhook signing (`whsec_...`) |
| `ironflyer:stripePricePro` / `Team` / `Enterprise` | Stripe price IDs (`price_...`) |
| `ironflyer:paddleApiKey` | Paddle billing API key |
| `ironflyer:paddleWebhookSecret` | Paddle webhook signing secret |
| `ironflyer:paddlePriceBuilder` / `Business` | Paddle price IDs |
| `ironflyer:githubClientID` / `ClientSecret` | GitHub OAuth App |
| `ironflyer:githubAppPrivateKey` | `cat ironflyer-app.pem` |
| `ironflyer:githubAppWebhookSecret` | GitHub App webhook secret |
| `ironflyer:resendApiKey` | Resend transactional email (`re_...`) |
| `ironflyer:glitchtipDatabaseUrl` | Managed-Postgres DSN for the `glitchtip` database (see step 5) |
| `ironflyer:superuserEmail` | initial admin user |
| `ironflyer:superuserPassword` | initial admin password (rotate after first login) |

Optional providers (only set if enabled):

| Key | Source |
| --- | --- |
| `ironflyer:openaiApiKey` | OpenAI |
| `ironflyer:geminiApiKey` | Google AI Studio |
| `ironflyer:hfApiKey` | HuggingFace |
| `ironflyer:deepseekApiKey` | DeepSeek |
| `ironflyer:vercelAiGatewayToken` | Vercel AI Gateway |
| `ironflyer:datadogApiKey` | Datadog metrics export |

Verify the set is complete:

```bash
pulumi config | sort
# Sanity check: every "required" row above must appear.
```

If anything is missing, `pulumi preview` will surface it as a
`Pulumi config 'X' is required` error — fix and retry.

### 3. Paddle merchant account

- Create a Paddle Billing account at <https://vendors.paddle.com/>.
- Enable production mode (KYC).
- Create products + prices matching the table above.
- Set the webhook URL to `https://ironflyer.ai/api/webhook/paddle`.
- Copy the API key + webhook signing secret into Pulumi config.
- Smoke-test against the sandbox before flipping the env to
  production (`PADDLE_ENV=production`).

### 4. Stripe live keys

- `sk_live_...` secret key.
- `whsec_...` webhook signing secret (webhook URL =
  `https://ironflyer.ai/budget/webhook`).
- `price_...` IDs for Pro, Team, Enterprise tiers.

Verify by hitting the Stripe API once with the live key:

```bash
curl -sS https://api.stripe.com/v1/balance \
  -u "${STRIPE_LIVE_KEY}:" | jq '.available[]?.amount // empty'
```

A non-error response means the key is alive. If you see
`Invalid API Key`, it is a test key.

### 5. GlitchTip — self-hosted Sentry-compatible error tracker

Ironflyer ships its own error tracker inside the Helm chart instead
of paying for Sentry SaaS. The DSN format is wire-identical, so the
`@sentry/*` and `sentry-go` SDKs we already use point at GlitchTip
without a single code change. Full runbook:
[`docs/OBSERVABILITY_GLITCHTIP.md`](OBSERVABILITY_GLITCHTIP.md).

Two pieces happen here in the preflight; the rest happens AFTER the
first `pulumi up` because the GlitchTip URL only exists post-deploy.

Now (preflight):

- Pre-create the `glitchtip` Postgres database + role on the managed
  DO Postgres cluster (see `OBSERVABILITY_GLITCHTIP.md § Database
  bootstrap`) and capture the DSN.
- Stash the DSN in Pulumi config so the Helm release picks it up:

  ```bash
  pulumi config set --secret ironflyer:glitchtipDatabaseUrl \
    "postgres://glitchtip:<pw>@<managed-host>:25060/glitchtip?sslmode=require"
  ```

After `pulumi up` (deferred to §2c § post-deploy):

- Visit <https://errors.ironflyer.ai>, sign up the first user
  (becomes superuser on a fresh install).
- Create the `ironflyer` organization, then create three projects:
  `orchestrator` (Go), `web` (JavaScript / Next.js),
  `vscode-extension` (Node).
- Copy each DSN into `.env.production.local`:

  ```bash
  SENTRY_DSN_ORCHESTRATOR=https://<key>@errors.ironflyer.ai/<id>
  SENTRY_DSN_WEB=https://<key>@errors.ironflyer.ai/<id>
  NEXT_PUBLIC_SENTRY_DSN=https://<key>@errors.ironflyer.ai/<id>
  SENTRY_DSN_VSCODE_EXTENSION=https://<key>@errors.ironflyer.ai/<id>
  ```

- Re-run the secrets loader + redeploy:

  ```bash
  bash scripts/load-secrets-to-pulumi.sh prod
  pulumi up
  ```

- Flip `glitchtip.enableUserRegistration: false` in
  `infra/helm/ironflyer/values-prod.yaml` and re-run `helm upgrade`
  to lock signups now that the admin exists.

### 6. GitHub App

- Create at <https://github.com/settings/apps/new>.
- Permissions: `contents:read`, `metadata:read`,
  `pull_requests:write` (the App is how Ironflyer pushes generated
  patches back to user repos).
- Generate + download the private key (`.pem`).
- Set the webhook URL to `https://ironflyer.ai/api/webhook/github`
  and the webhook secret to match the Pulumi config value.
- Copy the App ID + Client ID + Client Secret into Pulumi.

### 7. Resend domain verification

- Add a domain `ironflyer.ai` in Resend.
- Resend prints SPF / DKIM / DMARC records — add them as TXT
  records in the DO domain (or wait until §2c §pulumi up creates
  the zone, then add them via `doctl compute domain records create`).
- Confirm verification in the Resend dashboard before sending
  the first transactional email.

### 8. Anti-Bloat pre-deploy health (this wave's deliverable)

Run the Anti-Bloat tool drivers and inspect the reports BEFORE
applying:

```bash
./scripts/health/run-health.sh
ls -la tmp/reports/
```

- `tmp/reports/govulncheck-<ts>.json` — Go vulnerability scan.
- `tmp/reports/jscpd-<ts>.json` — TS/TSX duplication.

Wire the orchestrator gates by setting the report paths on the
running orchestrator (Pulumi reads them from the same secret
namespace; see `infra/helm/charts/orchestrator/values.yaml`):

```bash
pulumi config set ironflyer:vulnReportPath  "/run/reports/govulncheck-latest.json"
pulumi config set ironflyer:dedupReportPath "/run/reports/jscpd-latest.json"
```

`high`/`critical` Go vulns BLOCK deploy via the engine's gate
runner (SeverityCritical = refuse to apply). `medium` is a
Warning; `low` is Info. Dup > 2 % yields SeverityError on the
`dedup` gate.

---

## §2c — `pulumi up` walkthrough

This is the actual command sequence for a fresh prod install.
The runbook short version is
[`docs/RUNBOOKS/cold-start.md`](RUNBOOKS/cold-start.md).

```bash
cd infra/pulumi

# 1. Auth Pulumi (managed cloud or self-hosted backend).
pulumi login

# 2. Auth DigitalOcean. Either:
#    a) doctl: export the DO API token via doctl
doctl auth init
#    b) Or set DIGITALOCEAN_TOKEN directly:
export DIGITALOCEAN_TOKEN="dop_v1_..."

# 3. Pick the stack.
pulumi stack select prod            # ams3 region

# 4. Dry-run. READ THE DIFF.
pulumi preview

# 5. Apply. Cold install ≈ 20–30 minutes
#    (DOKS + managed PG + managed Redis are the slow steps).
pulumi up

# 6. Print the NS records and paste at the domain registrar.
pulumi stack output domainNameServers
#   Expected: ns1.digitalocean.com / ns2.digitalocean.com /
#             ns3.digitalocean.com (idempotent — already set in §2b
#             step 1; this output is the verification line).

# 7. Wire kubectl + watch pods settle.
doctl kubernetes cluster kubeconfig save "$(pulumi stack output doksClusterName)"
kubectl -n ironflyer get pods -w
#   Expect: orchestrator, runtime, web, code-server, audit-verify cron.
```

If `pulumi up` reports `cert-manager: HTTP-01 self-check failed`,
the DNS NS delegation is incomplete — re-verify §2b step 1.

---

## §2d — Post-deploy smoke

Run the V22 paid-execution smoke against the live URL:

```bash
IRONFLYER_API_URL=https://ironflyer.ai \
  ./scripts/v22_smoke.sh
```

Expected pass output:

```text
v22 smoke result: PASS
  wallet:           { "balanceUSD": ..., ... }
  execution:        { "id": "...", "status": "..." }
  profitDashboard:  { "revenueUSD": ..., "grossMarginPct": ... }
```

If you see `PASS-WITH-WARN`, read the warning — the typical cause
is a ledger read-path issue documented in §6 of `v22_smoke.sh`.

Optional secondary smoke (analytics ingestion):
[`DEPLOY.md §6.1`](../DEPLOY.md#61-analytics-ingestion-smoke-clickhouse--redpanda).

---

## §2e — Day-2 ops

Runbooks under [`docs/RUNBOOKS/`](RUNBOOKS/):

| Scenario | Runbook |
| --- | --- |
| First boot / restart-from-cold | [`cold-start.md`](RUNBOOKS/cold-start.md) |
| Rolling release | [`upgrade.md`](RUNBOOKS/upgrade.md) |
| Roll a release back | [`rollback.md`](RUNBOOKS/rollback.md) |
| Provider cost spike | [`cost-spike.md`](RUNBOOKS/cost-spike.md) |
| Workspace pool saturation | [`workspace-saturation.md`](RUNBOOKS/workspace-saturation.md) |
| GraphQL latency / error incident | [`graphql-incident.md`](RUNBOOKS/graphql-incident.md) |
| Analytics bring-up (ClickHouse + Redpanda) | [`analytics-bringup.md`](RUNBOOKS/analytics-bringup.md) |
| Temporal bring-up | [`temporal-bringup.md`](RUNBOOKS/temporal-bringup.md) |
| AppSec coverage | [`appsec-coverage.md`](RUNBOOKS/appsec-coverage.md) |
| First paid-customer launch | [`paid-customer-launch.md`](RUNBOOKS/paid-customer-launch.md) |

SLOs to monitor live: [`docs/PERF_BUDGETS.md`](PERF_BUDGETS.md).

---

## §2f — Known deferrals (honest)

The launch ships with these intentional gaps. None block commercial
launch on their own; each is the next reasonable improvement.

- **Time partitioning** for `ledger_entries`, `audit_log`,
  `execution_events`. The tables have indexes (see the 7 perf
  indexes); monthly partitioning lands after the first 5 M-row
  table hits its read-amplification ceiling.
- **Refactor Proposer / codemod tooling** — the Anti-Bloat Engine
  MVP detects bloat but does NOT auto-rewrite. The proposer that
  closes that loop is the next Anti-Bloat sprint.
- **Tool installs beyond govulncheck / jscpd** — `knip`, `gocognit`,
  `goleak`, `hyperfine`, `size-limit` are still evidence-stub
  gates: they go SeverityInfo "tool not installed" until the
  matching `scripts/lint/run-<tool>.sh` lands.
- ~~**Web bundle large PNGs** in `clients/web/public/brand/`~~ —
  Swept on 2026-05-26: 7 unreferenced files (ai-brain.jpg,
  ai-hand.png, ai-team.png, hero-developer.png, human-ai.png,
  ironflyer-cosmos.png, programming.png) deleted via `git rm`,
  ~5.0 MB recovered. `ironflyer-logo.svg` is the only remaining
  asset and is referenced from `clients/web/app/layout.tsx`.
- ~~**Mobile starters** at `templates/starters/{react-native-expo,
  android-kotlin,ios-swift}`~~ — Content-audited on 2026-05-26;
  see [`docs/MOBILE_STARTERS_AUDIT_2026-05-26.md`](MOBILE_STARTERS_AUDIT_2026-05-26.md).
  Versions are current (Expo SDK 53, AGP 8.7.2, Kotlin 2.0.21,
  Android SDK 35, Swift 5.10). Trivial fix applied to the iOS
  starter: added `Resources/PrivacyInfo.xcprivacy` (mandatory
  since 2024-Q1) and wired it into `xcodegen.yml`. Deferred
  follow-ups: `ITSAppUsesNonExemptEncryption` is an operator
  decision; SDK version bumps are intentional.
- **Capability Atlas search-hook** in the Coder agent is
  contractual today (the Preflight decision API ships, validation
  enforces a `reuse`/`extend`/`new` decision, and the indexer
  runs at boot) — but the actual model-side integration that
  CALLS the search lives as a one-pass follow-up.
---

## §2g — Definition of Done recap

From [`docs/PROJECT_CLOSEOUT_PLAN.md`](PROJECT_CLOSEOUT_PLAN.md):

| # | DoD item | Status after this wave | Note |
| - | --- | :-: | --- |
| 1 | User can top up, run, stop, refund, and inspect an execution | ✓ | `v22_smoke.sh` exercises all five paths end-to-end. |
| 2 | Every cost lands in the ledger | ✓ | BillingGuard writes are mandatory; the row-scan read bug closed earlier in the wave. |
| 3 | Every execution emits events into Redpanda | partial | Outbox publisher exists; Redpanda is `--profile analytics` (opt-in). Lean default still serves events from the in-process pub/sub. |
| 4 | ClickHouse dashboards show margin in near real time | partial | Async insert path landed; `profitDashboard` in lean dev still reads the in-process aggregator. Flip via `IRONFLYER_DB_DRIVER=hybrid`. |
| 5 | SurrealDB improves reuse/retrieval without owning durable truth | ✓ | Default in production profiles; durable truth stays in Postgres. |
| 6 | Temporal can resume interrupted workflows | partial | Temporal is `--profile temporal` (opt-in); the prod stack toggles it on. Lean dev is in-process. |
| 7 | Runtime workers scale by demand and shrink when idle | partial | HPA + topic-lag KEDA wiring exists in the Helm chart; verified manually under load on staging, not yet in continuous prod canary. |
| 8 | GraphQL is hardened for production | ✓ | Complexity + depth caps, APQ policy, masked errors, introspection off in `IRONFLYER_PROD=true`. |
| 9 | One synthetic paid execution passes in CI/CD | ✓ | `.github/workflows/ci.yml::go::smoke` runs `scripts/smoke.sh` (which tail-calls `v22_smoke.sh`) on every PR. |
| 10 | Operators can deploy, rollback, restore, and explain costs | ✓ | This document closes the deploy + rollback (RUNBOOKS) + restore (PITR) + cost-explain (ProfitDashboard + cost-spike runbook) loop. |

Net: 7 ✓, 3 partial, 0 ✗. The three `partial` lines are all the
"prod-profile-only" data plane services (Redpanda / ClickHouse /
Temporal) — they ship hot in `prod` and warm-cold in lean dev,
which is the intended split.

---

## Final go / no-go

You are clear to launch `ironflyer.ai` when ALL of the following
are true:

- [ ] §2b — every required Pulumi secret is set (`pulumi config | sort`
      shows the full table).
- [ ] §2b — `dig +short NS ironflyer.ai` returns the DO NS chain.
- [ ] §2b — `./scripts/health/run-health.sh` exits with no
      `high`/`critical` vuln finding (mediums are acceptable for
      launch; log them as follow-ups).
- [ ] §2c — `pulumi up` completes without `cert-manager` errors
      and `kubectl -n ironflyer get pods` shows every workload
      `Ready 1/1`.
- [ ] §2d — `./scripts/v22_smoke.sh` against the live URL prints
      `v22 smoke result: PASS` (PASS-WITH-WARN is acceptable only
      if the warn matches a known-deferral row in §2f).
- [ ] `https://ironflyer.ai/livez` returns 200.
- [ ] `https://ironflyer.ai/` renders the cockpit (not a 502
      bad gateway page).
- [ ] The first synthetic paid execution from the smoke shows up
      in the live profitDashboard with non-zero `grossMarginPct`.

When every box is checked, post the launch announcement and start
watching the SLOs in [`docs/PERF_BUDGETS.md`](PERF_BUDGETS.md).
