# Runbook — Rolling upgrade

End-to-end upgrade for an already-installed stack. Production path uses
Pulumi-managed Helm releases; the dev/local path is just a `go build` +
restart.

## Local / dev

```bash
# 1. Pull the new code.
git pull --ff-only

# 2. Rebuild + revet — must both exit 0.
cd apps/orchestrator && go build ./... && go vet ./...
cd ../runtime         && go build ./... && go vet ./...

# 3. Apply migrations against the live DB.
cd ../orchestrator
POSTGRES_URL="postgres://ironflyer:ironflyer@localhost:5432/ironflyer?sslmode=disable" \
  go run ./cmd/migrate up

# 4. Restart the orchestrator + runtime (Ctrl-C the previous `go run`).
go run ./cmd/orchestrator   # in one tab
cd ../runtime && go run ./cmd/runtime  # in another tab

# 5. Refresh the web codegen (schema may have moved).
cd ../web && npm run codegen && npm run dev

# 6. Smoke.
bash scripts/v22_smoke.sh
```

`v22_smoke.sh` is the canonical "did the upgrade survive" gate — it
walks signUp → wallet → paid execution → executionFeed → wallet/ledger
/execution/profitDashboard read assertions. Exit 0 with `PASS` (or the
documented `PASS-WITH-WARN`) is required before declaring the upgrade
healthy.

## Production (Pulumi-managed Helm)

```bash
cd infra/pulumi
pulumi stack select <stack>            # e.g. prod-eu

# Bump the image tag.
pulumi config set ironflyer:imageTag v0.X.Y

# Preview the diff. Read it before proceeding.
pulumi preview

# Apply.
pulumi up

# Watch the rollout.
kubectl -n ironflyer rollout status deploy/orchestrator
kubectl -n ironflyer rollout status deploy/runtime
kubectl -n ironflyer rollout status deploy/web

# Smoke.
IRONFLYER_API_URL=https://api.<stack>.ironflyer.dev bash scripts/v22_smoke.sh
```

Promote canary → staging → prod in order. Do **not** skip rings.

## Pre-flight checks

1. `go build` + `go vet` both modules exit 0 against the new commit.
2. `apps/web && npx tsc --noEmit` exits 0.
3. `bash scripts/v22_smoke.sh` exits 0 against the previous-tagged
   stack (otherwise you're upgrading a broken baseline).
4. Pulumi `preview` shows no surprising deletes (especially in the
   `data/` slice — Aurora/RDS replacement is not a rolling change).
