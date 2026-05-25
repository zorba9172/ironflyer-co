# Ironflyer Infrastructure (Pulumi, Go, AWS)

This Pulumi program provisions the **compute + edge half** of Ironflyer on
AWS. The **data half** (RDS Aurora Postgres, ElastiCache Redis, S3 buckets,
KMS, EFS, SurrealDB, Secrets Manager, observability, External Secrets) lives
in the sibling `data/` Go package inside this same project and is wired into
`main.go` through the `compute.Network` / `compute.Cluster` / IRSA outputs.

> For the application-side operational picture (Redis event bus,
> workspace portability, provider circuit breakers, connection-pool
> tuning, HPA defaults, GraphQL subscription scale) see
> [`../../docs/SCALE.md`](../../docs/SCALE.md). This README covers the
> AWS substrate; SCALE.md covers what the orchestrator does with it.

> No tests, no kubectl apply, no Helm install from the CLI. Pulumi owns the
> end-to-end cluster lifecycle and the application Helm chart in
> `infra/helm/ironflyer/` is deployed by the deploy pipeline on top of the
> cluster Pulumi creates here.

## What gets provisioned

### Compute (`compute/`)
- **VPC**: 3 AZs, configurable /16 CIDR, 3× /20 public + 3× /20 private +
  3× /24 DB subnets, NAT gateways (one per AZ — single-NAT in dev), VPC
  flow logs, S3 gateway endpoint + ECR/STS/CloudWatch/Secrets Manager
  interface endpoints, shared SG referenced by the data layer.
- **IAM** (base): cluster role, node role, four managed policy attachments
  on the node role (worker, ECR readonly, CNI, SSM).
- **EKS**: cluster (configurable k8s version, default `1.30`), OIDC IRSA
  provider, two managed node groups (`orchestrator-pool` and `runtime-pool`
  with `dedicated=runtime:NoSchedule`), addons (`vpc-cni`, `coredns`,
  `kube-proxy`, `aws-ebs-csi-driver`, `aws-efs-csi-driver`), authenticated
  Kubernetes Provider, then Helm releases for cluster-autoscaler,
  metrics-server, aws-load-balancer-controller, optionally Karpenter.
- **Workload IRSA**: `orchestrator-sa`, `runtime-sa`, `backup-sa`.

### Edge (`edge/`)
- **Route53** hosted zone per stack.
- **ACM** certs (regional for ALB + us-east-1 for CloudFront), DNS-01.
- **CloudFront** distribution with two origins (ALB + S3 SPA), path-based
  cache behaviors, HTTP/2 + HTTP/3, TLS 1.2_2021, OAC.
- **WAFv2** Web ACL (CLOUDFRONT scope, us-east-1) with managed rule sets
  (Common, KnownBadInputs, SQLi), per-IP rate-limit (1000 req / 5 min
  → 429), optional allow-list IP set; logs to CloudWatch.
- **External-DNS** and **cert-manager** Helm releases (after the zone
  exists, so their IRSA policies are zone-scoped).
- **Vercel** project (Next.js dashboard) + production domain + runtime
  env vars, plus a Route53 `CNAME` pointing the dashboard hostname at
  `cname.vercel-dns.com`. See the [Vercel](#vercel) section below.

## Stacks

| Stack       | Region        | Single NAT | Sizes          | Notes |
|-------------|---------------|------------|----------------|-------|
| `dev`       | eu-west-1     | yes        | t3.medium / t3.large | one-AZ-tolerable |
| `staging`   | eu-west-1     | no         | m6i.large       | mirrors prod smaller |
| `prod-eu`   | eu-west-1     | no         | m6i.xlarge      | api.eu.ironflyer.dev |
| `prod-us`   | us-east-1     | no         | m6i.xlarge      | api.us.ironflyer.dev |
| `prod-il`   | il-central-1  | no         | m6i.xlarge      | api.il.ironflyer.dev |

VPC CIDRs are pre-allocated per stack (`10.10.0.0/16` → dev,
`10.20.0.0/16` → staging, `10.30/10.40/10.50` → prod-eu/us/il) so peering
between stacks never collides.

## Prerequisites

- Pulumi CLI ≥ v3.140
- AWS account credentials (`aws sso login` or matching env vars)
- A Pulumi backend. Pulumi Cloud is the path of least resistance; for a
  self-hosted S3 + DynamoDB backend export:
  ```sh
  export PULUMI_BACKEND_URL=s3://ironflyer-pulumi-state
  pulumi login $PULUMI_BACKEND_URL
  ```
  The S3 + DynamoDB pair must exist beforehand — chicken-and-egg cycle.

## Deploy

```sh
cd infra/pulumi
go mod tidy && go build ./...

pulumi stack init dev
pulumi config set aws:region eu-west-1
pulumi up
```

For prod regions:

```sh
pulumi stack init prod-eu
pulumi up
```

The Pulumi.yaml stack config files are already populated with sensible
defaults — adjust `infra:publicApiCidrs`, `infra:allowlistedIps`, and
`infra:webSpaBucketName` per stack as needed.

## Cross-stack contract

The compute+edge side exports the following outputs (consumed by the data
package via the `Compute` struct in `main.go`, and re-exported as Pulumi
stack outputs for any downstream consumer):

| Output | Type | Purpose |
|--------|------|---------|
| `vpcId` | string | data layer's RDS / ElastiCache / EFS placement |
| `vpcCidr` | string | data layer's security-group rules |
| `privateSubnetIds` | string[] | EFS mount targets, internal LBs |
| `publicSubnetIds` | string[] | optional public ALB |
| `dbSubnetIds` | string[] | RDS cluster placement |
| `dbSubnetGroupId` | string | RDS cluster `db_subnet_group_name` |
| `eksClusterName` | string | identifies the cluster for downstream IRSA |
| `eksClusterEndpoint` | string | informational |
| `oidcProviderArn` | string | data-layer IRSA roles use this |
| `oidcProviderUrl` | string | data-layer IRSA roles use this |
| `hostedZoneId` | string | data-layer DNS records (e.g. `db.ironflyer.dev`) |
| `hostedZoneName` | string | informational |
| `certArn` | string | ALB listener cert (regional) |
| `certArnUsEast1` | string | CloudFront viewer cert |
| `orchestratorRoleArn` | string | application Pod role |
| `runtimeRoleArn` | string | runtime Pod role |
| `backupRoleArn` | string | backup CronJob role |

These are exposed both as `ctx.Export(...)` (so a peer Pulumi stack can read
them via `pulumi.NewStackReference`) and directly through the in-process
`data.Provision(...)` call.

## Tear-down notes

- NAT gateways and the CloudFront distribution are the biggest cost
  drivers; ALWAYS run `pulumi destroy` when an environment is no longer
  needed. NAT gateways alone run ≈ $33 / month each, so a 3-AZ prod stack
  is paying ≈ $100 / month before any traffic flows.
- The CloudFront distribution take ~15 minutes to destroy (it has to drain
  edge caches). Pulumi will block until AWS reports it disabled.
- The S3 buckets we create here (`cf-logs.ironflyer.${stack}`) are
  `ForceDestroy=false` to protect logs from accidental deletion. Empty them
  manually before `pulumi destroy` or set `ForceDestroy=true` temporarily.
- ECR repositories are NOT created by this program (image push happens in
  the deploy pipeline). If you teardown a stack, image data survives.

## Cost ballpark per stack

| Stack | Approx. monthly burn (idle) |
|-------|------------------------------|
| `dev` | ~$200 — single NAT, small nodes, EKS control plane ($72) |
| `staging` | ~$450 — 3-AZ NAT (~$100), m6i.large×6, EKS, ALB, CloudFront |
| `prod-eu` | ~$900 — full HA, m6i.xlarge×6, NAT×3, ALB, CloudFront, WAF |
| `prod-us` | ~$900 |
| `prod-il` | ~$900 (il-central-1 pricing) |

Numbers exclude the data layer (Postgres + Redis + EFS + observability) and
exclude egress and provider API spend.

## Vercel

The Ironflyer **dashboard** is a Next.js 15 app that ships on Vercel.
The split is intentional:

- **Vercel** runs the customer-facing dashboard (`apps/web`) — fast
  global edge, automatic preview deployments, and a clean separation
  from the backend's runtime concerns.
- **EKS** (provisioned by this Pulumi project + the `data/` package)
  runs the **orchestrator** (`apps/orchestrator`), the **runtime**
  workspace controller (`apps/runtime`), and every stateful piece
  (Postgres, Redis, SurrealDB, EFS).

The dashboard talks to the orchestrator's public hostname over HTTPS
+ WSS; the env vars (`NEXT_PUBLIC_IRONFLYER_API_URL`,
`NEXT_PUBLIC_IRONFLYER_WS_URL`, `NEXT_PUBLIC_SENTRY_DSN`) are written
into the Vercel project at production scope by `edge.NewVercel` so
the deploy artifact is self-contained.

### Required Pulumi config

| Key | Type | Notes |
|-----|------|-------|
| `ironflyer:vercelEnabled` | bool | Set to `true` to provision the dashboard for this stack. `false` for `dev` (runs locally). |
| `ironflyer:vercelTeamId` | string | Vercel team slug or ID. |
| `ironflyer:vercelDomain` | string | Public hostname, e.g. `app.eu.ironflyer.dev`. |
| `ironflyer:vercelBranch` | string | Production branch (default `main`). |
| `ironflyer:vercelFramework` | string | Framework preset (default `nextjs`). |
| `ironflyer:vercelGitRepoOwner` | string | GitHub owner of the dashboard repo (empty = no git binding). |
| `ironflyer:vercelGitRepoName` | string | GitHub repo name (empty = no git binding). |
| `ironflyer:vercelSentryDsn` | string | Optional override for `NEXT_PUBLIC_SENTRY_DSN`. Empty falls back to the data-layer Sentry secret ARN. |
| `vercel:apiToken` | **secret** | Vercel API token. **Always set with `--secret`.** |

Set the API token via:

```sh
pulumi config set --secret vercel:apiToken <token>
```

### Per-stack defaults

| Stack | `vercelEnabled` | Domain |
|-------|-----------------|--------|
| `dev` | `false` | `app.dev.ironflyer.dev` (off — run locally) |
| `staging` | `true` | `app.staging.ironflyer.dev` |
| `prod-eu` | `true` | `app.eu.ironflyer.dev` |
| `prod-us` | `true` | `app.us.ironflyer.dev` |
| `prod-il` | `true` | `app.il.ironflyer.dev` |

### Stack outputs

- `vercelProjectId` — the Vercel project ID (consumed by CI when
  triggering manual deploys via `vercel deploy --prod`).
- `vercelProductionURL` — `https://${ironflyer:vercelDomain}`.
- `vercelPreviewURLPattern` — handy for the orchestrator's preview
  inspector.

### Token relationship to the orchestrator's vercel-adapter

The orchestrator's `vercel-adapter` deploy target uses its own
`VERCEL_API_TOKEN` (it pushes generated user projects to Vercel).
That token is **separate** from the infra-side `vercel:apiToken`
used here, though in single-tenant prod both can point at the same
account / team. Keep them distinct in multi-tenant scenarios so a
compromised user-project token cannot mutate the dashboard project.

## Gotchas

- **CloudFront ACM certs must live in `us-east-1`**. The `tls.go` module
  builds a second cert in us-east-1 via the `usEast1` provider. Don't
  copy/paste it into the regional cert path.
- **WAF for CloudFront is also us-east-1**. The `waf.go` module asserts
  `Scope: CLOUDFRONT`.
- **OIDC trust policy thumbprint** is the well-known EKS root CA
  (`9e99a48a9960b14926bb7f3b02e22da2b0ab7280`). If AWS rotates it, the
  Pulumi state will drift and you'll need to update the constant in
  `compute/eks.go`.
- **External-DNS owner ID** is `ironflyer-${stack}` — DO NOT share zones
  between stacks unless you change this, or external-dns will delete
  records owned by the other stack.
- **Pulumi.yaml config requires `aws:region`** to be set before
  `pulumi up`. The stack config files in this directory already set it
  to the right default per environment.
- **Helm releases use the cluster's Pulumi provider** (no ambient
  kubeconfig). When `pulumi destroy` is run, the Helm releases destroy
  cleanly before the cluster is destroyed.
