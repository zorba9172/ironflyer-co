# Ironflyer — Production Stack

**Single source of truth: [`docker-compose.prod.yml`](./docker-compose.prod.yml).**

One Compose file deploys the full production stack on a Hetzner AX102
dedicated server. The companion AX42 runs the same file with
`IRONFLYER_ROLE=standby` for warm-standby HA. Stateless tiers
(orchestrator/web/runtime sandbox burst) run on Hetzner Cloud CCX23 nodes
provisioned by [`infra/pulumi/`](../pulumi/).

## What's in the box

| Capability | Service in compose | Port (internal) |
| --- | --- | --- |
| App DB + vector + relational | `postgres` (pgvector/pg:16) | 5432 |
| Knowledge graph | `surrealdb` (surrealkv backend) | 8000 |
| Cache + queues + Celery broker | `redis` (db=0 app, db=1 glitchtip) | 6379 |
| Object storage (S3-compat) | `minio` | 9000 / 9001 |
| Event stream *(opt-in: `--profile analytics`)* | `redpanda` | 9092 |
| Analytics OLAP *(opt-in: `--profile analytics`)* | `clickhouse` | 8123 / 9000 |
| Durable workflows | `temporal` + `temporal-ui` | 7233 / 8080 |
| Metrics | `victoriametrics` + `vmagent` | 8428 |
| Logs | `loki` + `promtail` | 3100 |
| Dashboards | `grafana` | 3000 |
| Error tracking | `glitchtip-web` + `glitchtip-worker` | 8000 |
| Edge / TLS / LB | `caddy` (auto Let's Encrypt) | 80 / 443 |
| App: API + GraphQL + SSE | `orchestrator-1`, `orchestrator-2` | 8080 |
| App: per-user sandboxes | `runtime` (docker driver + runsc) | 8090 |
| App: web UI | `web-1`, `web-2` (Next.js 15) | 3000 |
| Offsite backup | `restic` → Hetzner Storage Box | — |
| PG PITR | `wal-g` → MinIO + Storage Box | — |

## Lean default vs. scale-up

The stack ships **lean by default**. The OLAP analytics pipeline
(Redpanda + ClickHouse, ~20 GB RAM) is gated behind the `analytics`
Compose profile and is **not** started by a plain `up -d`. Nothing is
lost in lean mode: the orchestrator degrades gracefully — it skips the
event publisher and the profit / cohort / scale / blueprint dashboards
read **live Postgres** through their fallback adapters. The only thing
ClickHouse adds is faster aggregation at high event volume.

```bash
# Lean (recommended to start) — no Redpanda, no ClickHouse:
docker compose -f infra/compose/docker-compose.prod.yml \
  --env-file infra/compose/.env.prod up -d

# Scale up to OLAP analytics later (zero downtime; additive):
#   1. set REDPANDA_BROKERS + IRONFLYER_CLICKHOUSE_HOSTS in .env.prod
#   2. bring the two services up under the profile:
docker compose -f infra/compose/docker-compose.prod.yml \
  --env-file infra/compose/.env.prod --profile analytics up -d
```

Redis is **on by default** and load-bearing: with two orchestrator
replicas it provides the distributed finisher lock (prevents double
billing), the cross-pod event bus (subscriptions see events from either
pod), and shared rate-limit budgets. Dropping to a single orchestrator
is the only configuration where Redis is optional.

## Resource budget (AX102 — Ryzen 9 7950X3D, 128GB DDR5, 16c/32t)

| Tier | RAM committed | vCPU committed |
| --- | --- | --- |
| Postgres | 24 GB | 6.0 |
| ClickHouse *(opt-in: analytics)* | 14 GB | 4.0 |
| Redpanda *(opt-in: analytics)* | 6 GB | 2.5 |
| SurrealDB | 4 GB | 2.0 |
| Redis | 4 GB | 1.5 |
| MinIO | 4 GB | 2.0 |
| Temporal + UI | 4.5 GB | 2.5 |
| VictoriaMetrics + vmagent | 7 GB | 3.0 |
| Loki + Promtail + Grafana | 4.5 GB | 3.0 |
| GlitchTip web + worker | 4 GB | 2.0 |
| Orchestrator × 2 | 6 GB | 4.0 |
| Runtime + sandbox headroom | 8 GB + dynamic | 4.0 |
| Web × 2 | 2 GB | 2.0 |
| Caddy | 0.5 GB | 1.0 |
| **Total (lean default)** | **~72 GB** committed | **~33 vCPU** committed |
| **Total (+ analytics profile)** | **~92 GB** committed, **~36 GB** IO cache | **~40 vCPU** committed |

## Cold-start runbook (~30 minutes)

### Step 1 — provision the server

Order one **AX102** (Falkenstein FSN1) from
[Hetzner Robot](https://robot.hetzner.com/). Choose Ubuntu 24.04 LTS,
2× NVMe with software RAID1 mounted at `/mnt/data`.

### Step 2 — bootstrap the host

```bash
ssh root@<server-ip>
git clone https://github.com/zorba9172/ironflyer.git /opt/ironflyer
cd /opt/ironflyer
sudo bash scripts/host-bootstrap.sh --role primary --domain ironflyer.ai
```

Installs Docker + gVisor (`runsc`), opens 22/80/443/UDP-443, locks down
sysctls, creates the `/data` tree, restarts Docker with the gVisor runtime
registered.

### Step 3 — fill in secrets

```bash
cp infra/compose/.env.prod.example infra/compose/.env.prod
chmod 600 infra/compose/.env.prod
$EDITOR infra/compose/.env.prod
```

Generate the secrets that `.env.prod.example` lists with placeholders.
For each item ending in `CHANGE-ME`:

```bash
# 32-char hex (passwords)
openssl rand -hex 32
# 64-char hex (JWT secret)
openssl rand -hex 32          # 64 hex chars = 32 bytes
# Caddy basic-auth hash
docker run --rm caddy:2.8-alpine caddy hash-password --plaintext 'YOUR-PWD'
```

### Step 4 — DNS

Point the following hostnames at the AX102's public IP (one A record each):

```
ironflyer.ai            → A   <ax102-ip>
app.ironflyer.ai        → A   <ax102-ip>
api.ironflyer.ai        → A   <ax102-ip>
runtime.ironflyer.ai    → A   <ax102-ip>
s3.ironflyer.ai         → A   <ax102-ip>
grafana.ironflyer.ai    → A   <ax102-ip>
temporal.ironflyer.ai   → A   <ax102-ip>
minio.ironflyer.ai      → A   <ax102-ip>
errors.ironflyer.ai     → A   <ax102-ip>
vm.ironflyer.ai         → A   <ax102-ip>
```

If using Cloudflare, set each record to "DNS only" (gray cloud) until
Caddy completes the Let's Encrypt order — then flip to "Proxied" if you
want Cloudflare's WAF in front.

### Step 5 — bring it up

```bash
# Lean default (no Redpanda/ClickHouse). See "Lean default vs. scale-up".
docker compose -f infra/compose/docker-compose.prod.yml \
  --env-file infra/compose/.env.prod up -d
```

Watch the stack settle:

```bash
docker compose -f infra/compose/docker-compose.prod.yml ps
docker compose -f infra/compose/docker-compose.prod.yml logs -f caddy
```

Expect ~2–3 min for every service to report healthy. Caddy will provision
TLS certs in parallel on first request to each hostname.

### Step 6 — smoke

```bash
IRONFLYER_API_URL=https://api.ironflyer.ai \
  SMOKE_BEARER="$(curl -s -X POST https://api.ironflyer.ai/auth/token ...)" \
  ./scripts/smoke.sh
```

Then open https://app.ironflyer.ai and walk through a project creation +
sandbox spin-up to confirm the full E2E loop.

## Day-2 operations

### Apply config changes

```bash
docker compose -f infra/compose/docker-compose.prod.yml \
  --env-file infra/compose/.env.prod up -d --remove-orphans
```

Compose's `up -d` only restarts services whose config changed. Safe to run
repeatedly.

### Roll a new image

```bash
# .env.prod
IRONFLYER_VERSION=v0.43.0
```

```bash
docker compose -f infra/compose/docker-compose.prod.yml \
  --env-file infra/compose/.env.prod pull orchestrator-1 orchestrator-2 runtime web-1 web-2
docker compose -f infra/compose/docker-compose.prod.yml \
  --env-file infra/compose/.env.prod up -d \
  orchestrator-1 orchestrator-2 runtime web-1 web-2
```

Caddy stays up across the rolling restart — orchestrator-1 finishes
health checks before orchestrator-2 starts cycling, so requests keep
flowing.

### Rollback

Same as above with the previous `IRONFLYER_VERSION`. Database migrations
are forward-only — the orchestrator image is responsible for
schema-compatible reads against the prior schema for one version.

### Restore from backup

PG PITR is via WAL-G:

```bash
docker compose -f infra/compose/docker-compose.prod.yml \
  --env-file infra/compose/.env.prod \
  exec wal-g wal-g backup-list

docker compose -f infra/compose/docker-compose.prod.yml \
  --env-file infra/compose/.env.prod stop postgres
sudo rm -rf /data/postgres/*
docker compose -f infra/compose/docker-compose.prod.yml \
  --env-file infra/compose/.env.prod \
  run --rm wal-g wal-g backup-fetch /var/lib/postgresql/data LATEST
docker compose -f infra/compose/docker-compose.prod.yml \
  --env-file infra/compose/.env.prod start postgres
```

Cross-host DR is via restic (offsite Storage Box). See
[`restic/cron.sh`](./restic/cron.sh). Restore:

```bash
docker run --rm \
  -v /data:/data \
  -e RESTIC_REPOSITORY=$RESTIC_REPOSITORY \
  -e RESTIC_PASSWORD=$RESTIC_PASSWORD \
  restic/restic:0.17.3 \
  restore latest --target /
```

## Why this layout (slim wins vs the old Helm chart)

1. **Caddy** replaces ingress-nginx + cert-manager + external-dns +
   sealed-secrets. 40 MB binary, automatic TLS, no operator overhead.
2. **VictoriaMetrics** replaces kube-prometheus-stack. Same PromQL API,
   ~70 % less disk per series, ~50 % less RAM, no separate alertmanager
   process unless you need it.
3. **Loki single-binary** replaces the distributed loki-stack chart. One
   container, retention compactor built in.
4. **One Postgres** serves app + Temporal + GlitchTip via separate
   databases. Previously: 3 separate PG processes (managed + Temporal-PG +
   GlitchTip-PG).
5. **One Redis** serves the app (db=0) and GlitchTip's Celery broker
   (db=1). Previously: 2 separate Redis instances.
6. **No Kubernetes**. k3s overhead (kube-apiserver + scheduler +
   controller-manager + kubelet + 5 operators) is replaced by Compose's
   restart policies + healthchecks. ~8–12 GB RAM saved.
7. **Local NVMe via bind mounts** instead of CSI block storage over the
   network. PG/ClickHouse IOPS jump 10–20×.

## What the legacy paths are for now

- [`infra/helm/ironflyer/`](../helm/ironflyer/) — kept as a reference and
  for users who want to deploy on existing Kubernetes. Not the prod
  path anymore; the Compose file is the source of truth.
- [`infra/k8s/`](../k8s/) — same, kustomize bundles.
- [`infra/pulumi/compute/doks.go`](../pulumi/compute/doks.go) — replaced
  by `compute/hetzner.go` (see Pulumi section).

## Cost reference

| Item | Monthly |
| --- | --- |
| AX102 dedicated (this server) | €119 / ~$130 |
| AX42 warm-standby (runs the same compose with role=standby) | €44 / ~$48 |
| 2× CCX23 (stateless app pool, k3s/agent or direct compose) | €55 / ~$60 |
| 1× CCX23 (runtime baseline) | €27.50 / ~$30 |
| 0→1 CCX23 burst (autoscaler cap) | $0–30 |
| Hetzner Load Balancer LB11 | $6 |
| Storage Box BX21 (5 TB DR) | $12 |
| **Total** | **$286–316/mo** |

Egress: 20 TB/month included **per server**, so the full pair gets
80–120 TB of free outbound bandwidth — covers SSE LLM streaming and
sandbox bundle downloads with no overage.
