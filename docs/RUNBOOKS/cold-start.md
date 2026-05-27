# Runbook — Cold start (dev + prod)

This runbook is the minimum verified path from an empty machine to a
healthy Ironflyer stack. Every command below has been re-executed
against the live dev stack on 2026-05-26 and verified to exit 0 unless
explicitly marked otherwise.

The production install uses the single DigitalOcean Pulumi program at
`infra/pulumi/` (stack: `prod`); that path is the subject of
[`../../DEPLOY.md`](../../DEPLOY.md). This runbook focuses on the
local-dev cold start that operators actually rehearse against.

## 1. Prerequisites

- Docker Desktop 4.27+ (or compose v2)
- Go 1.23+
- Node 20+
- `psql` client (optional, for direct wallet seeding)

## 2. Start the lean infra

The default profile boots only what the orchestrator + runtime hard-
require: postgres (pgvector), redis, surrealdb, minio.

```bash
docker compose -f infra/compose/docker-compose.dev.yml up -d
docker compose -f infra/compose/docker-compose.dev.yml ps
```

Healthy state (verified 2026-05-26): postgres, redis, minio report
`healthy`; surrealdb reports `unhealthy` because the image's bundled
healthcheck queries an endpoint that returns 404, but the RPC at
`ws://localhost:8000/rpc` is live. Treat surrealdb `unhealthy` as a
known cosmetic warning, not a failure.

Optional profiles (opt-in, **not** part of the lean default):

```bash
# Analytics pipeline (Redpanda + ClickHouse, ~1.3 GB)
docker compose -f infra/compose/docker-compose.dev.yml --profile analytics up -d

# Durable workflows (Temporal + UI on :8233)
docker compose -f infra/compose/docker-compose.dev.yml --profile temporal up -d

# Stripe webhook forwarder (needs STRIPE_SECRET_KEY in env)
docker compose -f infra/compose/docker-compose.dev.yml --profile stripe up -d
```

## 3. Apply database migrations

```bash
cd core/orchestrator
POSTGRES_URL="postgres://ironflyer:ironflyer@localhost:5432/ironflyer?sslmode=disable" \
  go run ./cmd/migrate up
```

Verified output on a current dev DB:

```
goose: no migrations to run. current version: 41
migrations applied
```

## 4. Build / vet the Go modules

```bash
cd core/orchestrator && go build ./... && go vet ./...
cd ../runtime         && go build ./... && go vet ./...
```

Both modules MUST exit 0. If they don't, do not start the services.

## 5. Boot the services (host-side, fastest iteration)

```bash
# orchestrator on :8080
cd core/orchestrator && go run ./cmd/orchestrator

# runtime on :8090
cd core/runtime && go run ./cmd/runtime

# web on :3000 (requires npm install + codegen once)
cd clients/web && npm install && npm run codegen && npm run dev
```

The web codegen step pulls the schema from `http://localhost:8080/graphql`
so the orchestrator must already be running when you run `npm run codegen`.

## 6. Verify

```bash
# Root banner — confirms GraphQL is the API of record.
curl -s http://localhost:8080/ | jq .

# k8s probes.
curl -s http://localhost:8080/livez    | jq .
curl -s http://localhost:8080/readyz   | jq .
curl -s http://localhost:8080/version  | jq .

# GraphQL handshake.
curl -s -X POST http://localhost:8080/graphql \
  -H 'content-type: application/json' \
  -d '{"query":"{ __typename }"}'

# Runtime health.
curl -s http://localhost:8090/healthz | jq .

# Web.
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:3000/
```

Expected outputs (verified 2026-05-26):

- `GET /` returns
  `{"contract":"docs/V22_PLAN.md","graphql":"/graphql","sandbox":"/graphql/sandbox","service":"ironflyer-orchestrator","version":"dev"}`.
- `/livez`, `/readyz`, `/version`, `/healthz` all return 200 with JSON.
- The GraphQL handshake returns `{"data":{"__typename":"Query"}}`.
- The web returns 200.

## 7. End-to-end smoke

```bash
bash scripts/smoke.sh
bash scripts/v22_smoke.sh
```

Current verified behavior (2026-05-26):

- `scripts/v22_smoke.sh` exits **0** with `PASS-WITH-WARN` — the warn is
  a known `ledger` GraphQL row-scan mismatch flagged in
  `core/orchestrator/internal/ledger/postgres.go`. The script itself
  exits 0 because the V22 economic contract (signUp → wallet → paid
  execution → wallet hold) all pass.
- `scripts/smoke.sh` currently exits **non-zero** in dev because three
  of its sections probe REST/GraphQL surfaces that are no longer wired
  in a clean dev boot:
    - `GET /projects` — REST route was removed (GraphQL-only repo).
    - `plans` / `providersHealth` queries return 422 unless the
      orchestrator is booted with the V22 plans + providers stores wired.
    - `verifyAudit` mutation returns 422 in dev without an operator JWT.
  These are smoke-script gaps, not regressions in the live stack.
  Rerun `smoke.sh` against staging where the V22 stores are wired and
  `SMOKE_BEARER` is exported.

## 8. Common cold-start gotchas

- **`POSTGRES_URL` unset** — the migrate command refuses to run. Export
  it before `go run ./cmd/migrate`.
- **Port 5432/6379/8000/9000 already in use** — another local postgres
  / redis / surrealdb / minio is bound. Stop the conflicting service
  before `docker compose up`.
- **`npm run codegen` fails with `ECONNREFUSED 127.0.0.1:8080`** — the
  orchestrator is not running. Boot it first.
- **`v22_smoke.sh` reports the ledger scan warn** — fix lives in
  `core/orchestrator/internal/ledger/postgres.go`; restart the
  orchestrator after `go build` to apply.

## 9. Production domain — `ironflyer.ai` (DigitalOcean registrar)

`ironflyer.ai` is registered at the DigitalOcean **registrar** but its
authoritative DNS is **Cloudflare** (see `infra/pulumi/edge/cloudflare.go`
— Cloudflare owns DNS + WAF; the DO DNS service is intentionally
unused so the WAF + proxying are uniform).

Before the first `pulumi up` against the `prod` stack the operator MUST
delegate the zone to Cloudflare at the DigitalOcean registrar:

1. In the Cloudflare dashboard, add the zone `ironflyer.ai`. Cloudflare
   assigns four name servers (the exact hostnames are zone-specific;
   typical pattern is `<word>.ns.cloudflare.com`). Capture all four.
2. Sign in to the DigitalOcean control panel → **Networking →
   Domains → ironflyer.ai → Manage Name Servers** (or use
   `doctl domains records create` against the **registrar** product,
   not the DNS product — the registrar API is under
   `https://api.digitalocean.com/v2/registrar/...`).
3. Replace the default DigitalOcean name servers
   (`ns1.digitalocean.com`, `ns2.digitalocean.com`,
   `ns3.digitalocean.com`) with the four Cloudflare NS hostnames from
   step 1.
4. Wait for propagation (typically 5–30 min; up to 48 h worst case).
   Verify with:
   ```bash
   dig +short NS ironflyer.ai
   # Expect four *.ns.cloudflare.com entries.
   ```
5. Once Cloudflare reports the zone as **Active**, run `pulumi up` in
   `infra/pulumi/` against the `prod` stack. The Pulumi program
   will create the `api.`, `runtime.`, `app.`, `docs.` records inside
   the now-delegated zone.

If the NS delegation is skipped, cert-manager's HTTP-01 ACME challenge
will fail on the first ingress provisioning and the orchestrator API will
not be reachable at `https://api.ironflyer.ai`.

**Apex (`ironflyer.ai`) handling.** The bare apex is owned by the Vercel
production project (`clients/web`); add `ironflyer.ai` as a production
domain inside the Vercel project and accept the Vercel apex-record
recommendation (A 76.76.21.21 or the apex-alias CNAME if Cloudflare
flattening is on). The Pulumi program does not manage the apex record
— it only manages the four subdomains listed above.
