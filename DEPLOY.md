# Ironflyer — Production Deploy

End-to-end runbook for taking an empty cloud account to live Ironflyer
traffic. The application Helm chart is cloud-agnostic; the Pulumi
program you run picks the substrate. Two cloud paths are supported,
both production-grade:

| Cloud           | Pulumi program       | Substrate                                                                                                                                  | Cold-start runbook                                                                                  |
| --------------- | -------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------- |
| AWS             | `infra/pulumi/`      | VPC + EKS + Aurora Postgres + ElastiCache Redis + S3 + Route53 + ACM + CloudFront + WAFv2 + Vercel edge                                    | [`docs/RUNBOOKS/cold-start.md`](docs/RUNBOOKS/cold-start.md)                                        |
| DigitalOcean    | `infra/pulumi-do/`   | VPC + DOKS + Managed Postgres + Valkey (managed Redis) + Spaces + Cloudflare DNS/WAF + cert-manager + sealed-secrets + ingress-nginx + Vercel | follows the same Pulumi shape — config → `pulumi up` → smoke (see [`docs/RUNBOOKS/cold-start.md`](docs/RUNBOOKS/cold-start.md) §1–§7) |

The narrative below walks the **AWS path** in detail. The DO path is
the same shape end-to-end (Pulumi config → `pulumi up` → smoke).
Both paths read `RESEND_API_KEY` for transactional email — verify the
production domain at Resend before traffic flows regardless of cloud.

The application is layered on top via the **Helm chart** that the
Pulumi program installs as a Kubernetes resource — identical chart,
identical values surface, on both clouds.

If you're paging in mid-incident, jump straight to the focused
runbooks under [`docs/RUNBOOKS/`](docs/RUNBOOKS/):

- [`cold-start.md`](docs/RUNBOOKS/cold-start.md) — first install / fresh boot
- [`upgrade.md`](docs/RUNBOOKS/upgrade.md) — rolling upgrade
- [`rollback.md`](docs/RUNBOOKS/rollback.md) — rollback
- [`region-failover.md`](docs/RUNBOOKS/region-failover.md) — region failover
- [`cost-spike.md`](docs/RUNBOOKS/cost-spike.md) — provider cost spike
- [`workspace-saturation.md`](docs/RUNBOOKS/workspace-saturation.md) — sandbox saturation
- [`graphql-incident.md`](docs/RUNBOOKS/graphql-incident.md) — GraphQL surface broken

This document is the **first install** + **happy-path upgrade/rollback**
guide.

## TL;DR

- Pick a stack: `dev`, `staging`, `prod-eu`, `prod-us`, or `prod-il`.
- Set Pulumi config — secrets via `pulumi config set --secret`,
  non-secrets via plain `pulumi config set`. See § 4 for the full table.
- `cd infra/pulumi && pulumi stack select <stack> && pulumi up`.
- Wait for the Helm release to settle: `kubectl -n ironflyer get pods`
  should report every pod `1/1 Running`.
- Run smoke: `IRONFLYER_API_URL=... SMOKE_BEARER=... scripts/smoke.sh`.

## 1. Prerequisites

| Need | Why | Where |
| --- | --- | --- |
| AWS account + admin IAM | Pulumi provisions VPC/EKS/RDS/Redis/S3/Route53/ACM/CloudFront/WAF | AWS Organizations |
| Pulumi CLI ≥ 3.110 | runs the Go program | `brew install pulumi/tap/pulumi` |
| `kubectl` ≥ 1.28 | post-install verification | `brew install kubectl` |
| `helm` ≥ 3.13 | inspecting / debugging the chart Pulumi installs | `brew install helm` |
| `aws` CLI v2 | kubeconfig via `aws eks update-kubeconfig` | `brew install awscli` |
| Registered domain | Route53 hosted zone is provisioned by Pulumi; you delegate NS records at your registrar | Cloudflare / Gandi / Route53 Registrar |
| GitHub PAT | GitHub App webhook secret + `Continue with GitHub` OAuth | https://github.com/settings/applications/new |
| Stripe live keys | `/budget/checkout` + Stripe webhook | https://dashboard.stripe.com/ |
| Anthropic API key | default provider | https://console.anthropic.com/ |
| OpenAI / Gemini / HuggingFace / DeepSeek / Vercel AI Gateway | any subset of the additional providers you want enabled | each vendor's console |
| Vercel API token | edge cache + preview deploys for `apps/web` | https://vercel.com/account/tokens |
| Resend API key | transactional email (signup, password reset) | https://resend.com/api-keys |
| Sentry DSN | error capture (Go + Next.js) | https://sentry.io/ |

Only Anthropic + Stripe + GitHub OAuth are hard requirements for a
useful production stack. The other AI providers are optional — the
bandit happily runs with a single provider.

## 2. Stack layout

Pulumi stacks under `infra/pulumi/`:

| Stack | Region | Purpose |
| --- | --- | --- |
| `dev` | eu-west-1 | Throwaway. Single-NAT, smallest nodes. |
| `staging` | eu-west-1 | Pre-prod canary. |
| `prod-eu` | eu-west-1 | EU production. |
| `prod-us` | us-east-1 | US production. |
| `prod-il` | il-central-1 | IL production. |

Per-stack config lives in `infra/pulumi/Pulumi.<stack>.yaml`. The
Pulumi program composes three slices:

- **Compute** (`compute/`) — VPC, EKS, IAM/IRSA, cluster autoscaling,
  AWS Load Balancer Controller.
- **Data** (`data/`) — Aurora Postgres, ElastiCache Redis, S3 buckets,
  KMS, Secrets Manager, EFS, SurrealDB, External Secrets,
  kube-prometheus-stack + Loki.
- **Edge** (`edge/`) — Route53, ACM, CloudFront, WAFv2, external-dns,
  cert-manager.

The application Helm chart at [`infra/helm/ironflyer/`](infra/helm/ironflyer/)
is installed by Pulumi as a `helm.v3.Release` resource — there is no
separate `helm install` step in steady state. Pulumi owns the release.

> **Note on `infra/pulumi-data/`.** That separate program is the
> data-only managed-service path: it provisions RDS + Redis + S3 without
> EKS for operators who want to bring their own compute. The main
> [`infra/pulumi/`](infra/pulumi/) program is what this guide uses.

## 3. Stack outputs you'll consume

After `pulumi up`, `pulumi stack output` exposes:

- `kubeconfig` — write to `~/.kube/config-ironflyer-<stack>` and export
  `KUBECONFIG`.
- `apiURL` / `webURL` / `runtimeURL` — public URLs the Helm release is
  reachable at.
- `postgresEndpoint` / `redisEndpoint` / `s3Bucket` — for out-of-band
  ops work (psql, redis-cli, snapshots).
- `route53ZoneID` — delegate this zone's NS records at your registrar
  before traffic flows.

## 4. Cold-start install

The narrative walkthrough is below. The short, verified-against-the-
live-stack version for first install + daily ops lives at
[`docs/RUNBOOKS/cold-start.md`](docs/RUNBOOKS/cold-start.md).

```bash
# 1. Auth Pulumi (managed backend or self-hosted).
pulumi login                  # or: pulumi login s3://your-state-bucket

# 2. Auth AWS (named profile recommended).
export AWS_PROFILE=ironflyer-prod
aws sts get-caller-identity   # sanity check

# 3. Select the stack.
cd infra/pulumi
pulumi stack select prod-eu   # or dev / staging / prod-us / prod-il
```

Set non-secret config:

```bash
pulumi config set aws:region                  eu-west-1
pulumi config set ironflyer:domain            ironflyer.dev
pulumi config set ironflyer:imageRegistry     ghcr.io/zorba9172
pulumi config set ironflyer:imageTag          v0.42.0
pulumi config set ironflyer:replicas          3
```

Set secrets — every one of these is enumerated so nothing is implicit:

| Key | Source | Required? |
| --- | --- | --- |
| `ironflyer:jwtSecret` | `openssl rand -hex 32` | yes |
| `ironflyer:anthropicApiKey` | Anthropic console | yes |
| `ironflyer:stripeSecretKey` | Stripe live key | yes |
| `ironflyer:stripeWebhookSecret` | Stripe webhook signing secret | yes |
| `ironflyer:stripePricePro` | Stripe price ID, Pro tier | yes |
| `ironflyer:stripePriceTeam` | Stripe price ID, Team tier | yes |
| `ironflyer:stripePriceEnterprise` | Stripe price ID, Enterprise tier | yes |
| `ironflyer:githubClientID` | GitHub OAuth App | yes |
| `ironflyer:githubClientSecret` | GitHub OAuth App | yes |
| `ironflyer:githubAppPrivateKey` | GitHub App (webhook receiver) | yes |
| `ironflyer:githubAppWebhookSecret` | GitHub App | yes |
| `ironflyer:openaiApiKey` | OpenAI console | optional |
| `ironflyer:geminiApiKey` | Google AI Studio | optional |
| `ironflyer:hfApiKey` | HuggingFace | optional |
| `ironflyer:deepseekApiKey` | DeepSeek | optional |
| `ironflyer:vercelAiGatewayToken` | Vercel AI Gateway | optional |
| `ironflyer:vercelApiToken` | Vercel project deploy | yes if `apps/web` deploys to Vercel |
| `ironflyer:resendApiKey` | Resend transactional email | yes |
| `ironflyer:sentryDsnOrchestrator` | Sentry Go project | yes |
| `ironflyer:sentryDsnWeb` | Sentry Next.js project | yes |
| `ironflyer:datadogApiKey` | Datadog (optional metrics export) | optional |

```bash
# Each secret. Repeat per row in the table above.
pulumi config set --secret ironflyer:jwtSecret               "$(openssl rand -hex 32)"
pulumi config set --secret ironflyer:anthropicApiKey         sk-ant-...
pulumi config set --secret ironflyer:stripeSecretKey         sk_live_...
pulumi config set --secret ironflyer:stripeWebhookSecret     whsec_...
pulumi config set --secret ironflyer:stripePricePro          price_...
pulumi config set --secret ironflyer:stripePriceTeam         price_...
pulumi config set --secret ironflyer:stripePriceEnterprise   price_...
pulumi config set --secret ironflyer:githubClientID          Iv1...
pulumi config set --secret ironflyer:githubClientSecret      ...
pulumi config set --secret ironflyer:githubAppPrivateKey     "$(cat ironflyer-app.pem)"
pulumi config set --secret ironflyer:githubAppWebhookSecret  ...
pulumi config set --secret ironflyer:resendApiKey            re_...
pulumi config set --secret ironflyer:sentryDsnOrchestrator   https://...
pulumi config set --secret ironflyer:sentryDsnWeb            https://...
# Optional providers — only the ones you've enabled.
pulumi config set --secret ironflyer:openaiApiKey            sk-...
pulumi config set --secret ironflyer:geminiApiKey            AI...
pulumi config set --secret ironflyer:hfApiKey                hf_...
pulumi config set --secret ironflyer:deepseekApiKey          sk-...
pulumi config set --secret ironflyer:vercelAiGatewayToken    ...
pulumi config set --secret ironflyer:vercelApiToken          ...
```

Run the install:

```bash
# Dry-run — read the diff before you commit.
pulumi preview

# Apply.
pulumi up
```

Pulumi provisions cloud resources first, then installs the Helm release.
On a cold stack expect 18–25 minutes (EKS control plane + Aurora cluster
are the slow steps).

Delegate the Route53 NS records at your registrar before the cert-manager
order completes, or Let's Encrypt will fail HTTP-01:

```bash
pulumi stack output route53NameServers
# Paste each NS record into your registrar's NS configuration.
```

Wire kubectl to the new cluster and watch the chart settle:

```bash
aws eks update-kubeconfig --name "$(pulumi stack output eksClusterName)" \
  --region "$(pulumi stack output awsRegion)"

kubectl -n ironflyer get pods -w
# Expect: orchestrator, runtime, web, code, postgres-sidecar (if any),
#         redis-sentinel (if used), audit-verify (cron).
```

Run the post-install smoke (§ 6).

## 5. GraphQL endpoint

The orchestrator's API of record is **GraphQL**. After install:

| Surface | Path | Notes |
| --- | --- | --- |
| Queries + mutations | `POST /graphql` | `content-type: application/json` |
| Persisted queries / introspection GET | `GET /graphql` | APQ |
| Subscriptions | `WS /graphql` | Subprotocol `graphql-transport-ws` |
| Live documentation | `GET /graphql/sandbox` | Embedded Apollo Sandbox |

Authentication: `Authorization: Bearer <jwt>` on HTTP; on WS, send the
token in `connection_init.payload.authorization` as `Bearer <jwt>`.
Browsers that can't set `connection_init` may fall back to `?token=`.

APQ runs with an in-memory LRU; once your client registry is populated,
set `GRAPHQL_APQ_LOCKED=true` so ad-hoc queries from production clients
start erroring as designed.

GraphQL env knobs that govern the surface (all optional):

| Variable | Default | What it does |
| --- | --- | --- |
| `GRAPHQL_TRACING` | `off` | Apollo tracing extension. Verbose — flip on while debugging. |
| `GRAPHQL_COMPLEXITY_LIMIT` | `1000` *(planned)* | Hard cap on operation complexity. |
| `GRAPHQL_DEPTH_LIMIT` | `15` *(planned)* | Hard cap on selection-set depth. |
| `GRAPHQL_INTROSPECTION` | `on` | Set `off` once clients are on a generated SDK. |
| `GRAPHQL_APQ_LRU_SIZE` | `100` | In-memory APQ LRU. Raise on APQ miss-rate. |
| `GRAPHQL_APQ_LOCKED` | `false` *(planned)* | Reject non-registered hashes. |
| `GRAPHQL_QUERY_CACHE_SIZE` | `1000` | LRU size for parsed query documents. |

For deeper GraphQL ops (incident triage, subscription handshakes, APQ
locking) see [`docs/RUNBOOKS/graphql-incident.md`](docs/RUNBOOKS/graphql-incident.md).
The schema itself is the single source of truth — see
`apps/orchestrator/internal/graph/schema/*.graphql`.

## 6. Smoke

There are two smoke scripts. Run both after every `pulumi up`.

```bash
# Canonical V22 paid-execution contract — signUp → wallet → paid
# execution → executionFeed → wallet/ledger/execution/profitDashboard.
# Exits 0 on the happy path (PASS-WITH-WARN is also exit 0; the warn
# is the documented ledger row-scan mismatch).
IRONFLYER_API_URL=https://api.ironflyer.dev bash scripts/v22_smoke.sh

# Broader smoke — infra probes + GraphQL handshake + sample reads +
# subscription handshake + REST deprecation banner. Auth-gated
# sections warn-skip when SMOKE_BEARER is unset.
IRONFLYER_API_URL=https://api.ironflyer.dev \
SMOKE_BEARER=<operator-jwt> \
  bash scripts/smoke.sh
```

`v22_smoke.sh` is the deploy gate. `smoke.sh` is the broader
operability gate; some of its sections (e.g. `plans`,
`providersHealth`, `verifyAudit`, `/projects`) require optional V22
stores or REST surfaces that are not present on every build — those
sections may report `FAIL` against a dev box without those stores
wired. Treat `v22_smoke.sh` PASS as the hard prod gate; treat
`smoke.sh` failures as triage data for the gap, not a deploy block,
unless the failing surface is in scope for the stack you just
shipped.

Also gate every deploy on [`scripts/verify-headers.sh`](scripts/verify-headers.sh)
which checks HSTS / CSP / X-Frame-Options / X-Content-Type-Options /
Referrer-Policy / Permissions-Policy on both surfaces.

## 7. Upgrade (rolling)

Full runbook with pre-checks and verification:
[`docs/RUNBOOKS/upgrade.md`](docs/RUNBOOKS/upgrade.md).

Happy path:

```bash
# 1. Bump the image tag in Pulumi config.
cd infra/pulumi
pulumi stack select prod-eu
pulumi config set ironflyer:imageTag v0.42.1

# 2. Apply.
pulumi up

# 3. Watch the rollout.
kubectl -n ironflyer rollout status deploy/orchestrator
kubectl -n ironflyer rollout status deploy/runtime
kubectl -n ironflyer rollout status deploy/web

# 4. Smoke.
IRONFLYER_API_URL=https://api.ironflyer.dev bash scripts/v22_smoke.sh
```

Canary first: bump the tag on `staging`, soak for 30 minutes against
real traffic, then promote to `prod-eu` → `prod-us` → `prod-il`.

## 8. Rollback

Full runbook with database-migration considerations:
[`docs/RUNBOOKS/rollback.md`](docs/RUNBOOKS/rollback.md).

Fast path — re-pin the image tag and `pulumi up`:

```bash
pulumi config set ironflyer:imageTag v0.42.0   # the previous-known-good
pulumi up
```

Stack-history path — revert all Pulumi-managed resources to a prior
version:

```bash
pulumi stack history
pulumi stack export --version <N> > stack-vN.json
pulumi stack import --file stack-vN.json
pulumi up
```

Data caveat: Aurora and S3 are stateful. Reverting a Pulumi version
that touches `data/` is *not* automatically reversible. Schema
migrations are managed by the `apps/orchestrator/cmd/migrate` (goose-
backed) binary; rolling them back is the `migrate down` path and
should be done **after** the application is back on the previous tag.

## 9. Cross-references

- [`docs/RUNBOOKS/`](docs/RUNBOOKS/) — focused incident runbooks (cold-
  start, upgrade, rollback, region-failover, cost-spike,
  workspace-saturation, graphql-incident).
- [`ARCHITECTURE.md`](ARCHITECTURE.md) — V22 locked spec.
- [`QUICKSTART.md`](QUICKSTART.md) — fastest dev path from clone to
  paid execution.
- [`docs/V22_PLAN.md`](docs/V22_PLAN.md) — implementation contract.
- [`docs/PROJECT_CLOSEOUT_PLAN.md`](docs/PROJECT_CLOSEOUT_PLAN.md) —
  Definition of Done + verified state.
- `apps/orchestrator/internal/graph/schema/*.graphql` — GraphQL schema,
  single source of truth.
- [`infra/pulumi/README.md`](infra/pulumi/README.md) — cross-stack
  output contract.
- [`.github/workflows/ci.yml`](.github/workflows/ci.yml) — image build
  pipeline.
