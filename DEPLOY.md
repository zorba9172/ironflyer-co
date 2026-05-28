# Ironflyer — Production Deploy

End-to-end runbook for taking empty Hetzner servers to live Ironflyer
traffic.

**The production stack is defined in a single source-of-truth file:
[`infra/compose/docker-compose.prod.yml`](infra/compose/docker-compose.prod.yml).**
That Compose file is the contract; Pulumi only provisions the servers
the file runs on.

Substrate: Hetzner dedicated (AX102 + AX42) + Hetzner Cloud (3–4× CCX23)
+ Hetzner LB11 + Hetzner Storage Box + Cloudflare DNS/WAF (free tier).
TLS terminates at Caddy on each app node — no separate cert-manager /
ingress-nginx / external-dns / sealed-secrets layer.

For the full per-service runbook (resource limits, day-2 ops, restore
from backup, slim-wins explanation), read
[`infra/compose/README.prod.md`](infra/compose/README.prod.md). That
document is the canonical operational guide; this file is the
**first install** + **happy-path upgrade/rollback** index.

## Production security hardening

Before any prod traffic, work through
[`docs/SECURITY_HARDENING_2026-05-26.md`](docs/SECURITY_HARDENING_2026-05-26.md) §6
(the env-var checklist). The orchestrator fails-fast at startup when
`IRONFLYER_ENV=prod` and any of `IRONFLYER_JWT_SECRET`,
`IRONFLYER_CORS_ORIGINS`, or `IRONFLYER_METRICS_TOKEN` is missing or
unsafe. All three are wired in the production
[`.env.prod.example`](infra/compose/.env.prod.example).

## Incident runbooks

If you're paging in mid-incident, jump to:

- [`cold-start.md`](docs/RUNBOOKS/cold-start.md) — first install / fresh boot
- [`upgrade.md`](docs/RUNBOOKS/upgrade.md) — rolling upgrade
- [`rollback.md`](docs/RUNBOOKS/rollback.md) — rollback
- [`cost-spike.md`](docs/RUNBOOKS/cost-spike.md) — provider cost spike
- [`workspace-saturation.md`](docs/RUNBOOKS/workspace-saturation.md) — sandbox saturation
- [`graphql-incident.md`](docs/RUNBOOKS/graphql-incident.md) — GraphQL surface broken

The cold-start runbook is the verbatim verified-against-the-live-stack
version of what's below.

## TL;DR

1. Order **1× AX102** (primary) + **1× AX42** (warm-standby) from
   [Hetzner Robot](https://robot.hetzner.com/) — Falkenstein, Ubuntu 24.04.
2. `cd infra/pulumi && pulumi stack select prod && pulumi up` — provisions
   the 3× CCX23 cloud nodes, LB11, vSwitch network, Cloudflare DNS records.
3. On each server: `sudo bash scripts/host-bootstrap.sh --role <primary|standby|stateless|runtime> --domain ironflyer.ai`.
4. Copy `infra/compose/.env.prod.example` → `infra/compose/.env.prod`,
   fill in secrets (`chmod 600`).
5. On the AX102: `docker compose -f infra/compose/docker-compose.prod.yml --env-file infra/compose/.env.prod up -d`.
   This is the **lean default** — Redpanda + ClickHouse are opt-in via
   `--profile analytics` (dashboards read live Postgres without them). See
   [infra/compose/README.prod.md](infra/compose/README.prod.md#lean-default-vs-scale-up).
6. Wait for `docker compose ps` to show every service healthy (~3 min).
7. Smoke: `IRONFLYER_API_URL=https://api.ironflyer.ai SMOKE_BEARER=… scripts/smoke.sh`.

## 1. Prerequisites

| Need | Why | Where |
| --- | --- | --- |
| Hetzner account (Cloud + Robot) | Servers, LB, Storage Box | https://accounts.hetzner.com/ |
| Cloudflare account | DNS + WAF (free tier) for `ironflyer.ai` | https://dash.cloudflare.com/ |
| Pulumi CLI ≥ 3.110 | Provisions the Cloud nodes + DNS | `brew install pulumi/tap/pulumi` |
| Docker CE ≥ 27 (installed on servers by `host-bootstrap.sh`) | Runs the Compose stack | n/a (bootstrap installs) |
| Registered domain | Cloudflare hosts the zone | any registrar |
| GitHub PAT | OAuth + GitHub App | https://github.com/settings/applications/new |
| Stripe live keys | `/budget/checkout` + Stripe webhook | https://dashboard.stripe.com/ |
| Anthropic API key | Default provider | https://console.anthropic.com/ |
| OpenAI / Gemini / HuggingFace / DeepSeek / Vercel AI Gateway | Optional providers | each vendor's console |
| Resend API key | Transactional email | https://resend.com/api-keys |

Only Anthropic + Stripe + GitHub OAuth are hard requirements for a
useful production stack. The other AI providers are optional — the
bandit happily runs with a single provider.

## 2. Stack layout

| Layer | What | Where |
| --- | --- | --- |
| **Application stack** | All services (PG, Redis, Surreal, MinIO, Redpanda, ClickHouse, Temporal, Loki, VictoriaMetrics, Grafana, GlitchTip, orchestrator, runtime, web, Caddy) | [`infra/compose/docker-compose.prod.yml`](infra/compose/docker-compose.prod.yml) |
| **Edge routing** | Caddy with auto Let's Encrypt | [`infra/compose/Caddyfile.prod`](infra/compose/Caddyfile.prod) |
| **Secrets** | env file, chmod 600 | [`infra/compose/.env.prod.example`](infra/compose/.env.prod.example) |
| **Server provisioning** | Hetzner Cloud + LB + DNS via Pulumi | [`infra/pulumi/`](infra/pulumi/) (`prod` stack) |
| **Host bootstrap** | Docker + gVisor + sysctls + `/data` tree | [`scripts/host-bootstrap.sh`](scripts/host-bootstrap.sh) |
| **Backups** | WAL-G (PG → MinIO) + restic (everything → Storage Box) | [`infra/compose/restic/cron.sh`](infra/compose/restic/cron.sh) |

## 3. Cold-start install

### 3a. Order dedicated servers (manual)

Hetzner Robot does not have a first-class Pulumi provider. Order via
the web console:

- **AX102** — Ryzen 9 7950X3D, 128GB DDR5, 2× 1.92TB NVMe, Falkenstein
- **AX42** — Ryzen 5 7600, 64GB DDR5, 2× 512GB NVMe, Falkenstein

Both: Ubuntu 24.04 LTS, software RAID1 on the NVMe pair mounted at
`/mnt/data`. Note each server's **public IPv4** and the assigned
**private IP** on Hetzner's vSwitch (10.20.1.10 for AX102,
10.20.1.11 for AX42).

### 3b. Provision cloud tier via Pulumi

```bash
pulumi login                              # or self-hosted backend

cd infra/pulumi
pulumi stack select prod

# Drop in the dedicated-server IPs you just got.
pulumi config set ironflyer:statefulPrimaryIP   <ax102-public-ipv4>
pulumi config set ironflyer:statefulStandbyIP   <ax42-public-ipv4>

# Provider credentials.
pulumi config set --secret hetznerCloudToken    "$HCLOUD_TOKEN"
pulumi config set --secret cloudflareApiToken   "$CF_TOKEN"
pulumi config set --secret sshPublicKey         "$(cat ~/.ssh/id_ed25519.pub)"

pulumi preview                            # always read the diff first
pulumi up
```

This brings up: 2× CCX23 stateless + 1× CCX23 runtime baseline + 1×
LB11 + vSwitch network + Cloudflare A records for every subdomain
(`app`, `api`, `runtime`, `s3`, `grafana`, `temporal`, `minio`,
`errors`, `vm`).

### 3c. Bootstrap each server

```bash
# AX102 — primary
ssh root@<ax102-ip> 'curl -fsSL https://raw.githubusercontent.com/zorba9172/ironflyer/main/scripts/host-bootstrap.sh \
  | bash -s -- --role primary --domain ironflyer.ai'

# AX42 — standby
ssh root@<ax42-ip> 'curl -fsSL https://raw.githubusercontent.com/zorba9172/ironflyer/main/scripts/host-bootstrap.sh \
  | bash -s -- --role standby --domain ironflyer.ai'

# Each CCX23 — stateless / runtime
for ip in <stateless-1> <stateless-2>; do
  ssh root@$ip 'curl -fsSL ...host-bootstrap.sh | bash -s -- --role stateless --domain ironflyer.ai'
done
ssh root@<runtime-baseline-ip> '... --role runtime ...'
```

### 3d. Fill in `.env.prod` on the AX102

```bash
ssh root@<ax102-ip>
cd /opt/ironflyer
git clone https://github.com/zorba9172/ironflyer.git . || git pull
cp infra/compose/.env.prod.example infra/compose/.env.prod
chmod 600 infra/compose/.env.prod
vim infra/compose/.env.prod
```

Every placeholder ending in `CHANGE-ME` is required. Generate secrets:

```bash
openssl rand -hex 32                                            # 32-char hex (most passwords)
openssl rand -hex 32                                            # 64-hex-chars JWT secret
openssl rand -base64 36                                         # base64 36-char (PG, MinIO)
docker run --rm caddy:2.8-alpine caddy hash-password --plaintext 'YOUR-PWD'  # Caddy ops basic-auth
```

### 3e. Bring up the stack

```bash
docker compose -f infra/compose/docker-compose.prod.yml \
  --env-file infra/compose/.env.prod up -d
```

Watch settling:

```bash
docker compose -f infra/compose/docker-compose.prod.yml ps
docker compose -f infra/compose/docker-compose.prod.yml logs -f caddy
```

Caddy auto-orders Let's Encrypt certs for every hostname in
[`Caddyfile.prod`](infra/compose/Caddyfile.prod). Expect ~2 min until
every service reports healthy and certs are issued.

### 3f. Smoke

```bash
IRONFLYER_API_URL=https://api.ironflyer.ai \
  SMOKE_BEARER="$(curl -s -X POST https://api.ironflyer.ai/auth/token \
                   -d 'email=ops@ironflyer.ai&password=…' | jq -r .token)" \
  ./scripts/smoke.sh
```

Then open https://app.ironflyer.ai and walk through:

1. Sign up + log in.
2. Create a project with the Home prompt-first composer.
3. Open the Studio surface (visual state mirror).
4. Spin a sandbox via the runtime — confirm PTY WebSocket connects and
   gVisor isolation kicks in (`docker exec ironflyer-runtime docker ps`
   should show child containers with `Runtime: runsc`).

## 4. Upgrade

```bash
# .env.prod
IRONFLYER_VERSION=v0.43.0

docker compose -f infra/compose/docker-compose.prod.yml \
  --env-file infra/compose/.env.prod pull \
    orchestrator-1 orchestrator-2 runtime web-1 web-2

docker compose -f infra/compose/docker-compose.prod.yml \
  --env-file infra/compose/.env.prod up -d \
    orchestrator-1 orchestrator-2 runtime web-1 web-2
```

Caddy stays up across the rolling restart — `orchestrator-1` finishes
its `/healthz` before `orchestrator-2` cycles, so requests keep
flowing.

## 5. Rollback

Set `IRONFLYER_VERSION` to the previous tag, repeat § 4. Schema
migrations are forward-only — the orchestrator image promises a
single-version backward-compatible read.

## 6. Restore from backup

Postgres PITR via WAL-G + cross-host DR via restic — full procedure in
[`infra/compose/README.prod.md`](infra/compose/README.prod.md#restore-from-backup).

## 7. Legacy paths

Three previous deployment targets are kept in-tree as reference only.
None of them are the prod path anymore:

- [`infra/helm/ironflyer/`](infra/helm/ironflyer/) — Helm chart for
  Kubernetes deployments. Use only if you're running an existing k8s
  cluster and don't want the Compose path.
- [`infra/k8s/`](infra/k8s/) — Kustomize bundles. Same caveat.
- [`infra/pulumi/compute/doks.go`](infra/pulumi/compute/doks.go) +
  [`infra/pulumi/data/postgres.go`](infra/pulumi/data/postgres.go) +
  [`infra/pulumi/data/redis.go`](infra/pulumi/data/redis.go) +
  [`infra/pulumi/data/spaces.go`](infra/pulumi/data/spaces.go) +
  [`infra/pulumi/data/observability.go`](infra/pulumi/data/observability.go) —
  DigitalOcean DOKS + Managed PG/Valkey/Spaces. Superseded by Hetzner +
  Compose. Keep until the Pulumi rewrite lands; the `prod` stack
  currently fails when these files are wired in. Use the slim
  Hetzner-only program (TBD: `infra/pulumi/compute/hetzner.go`).
