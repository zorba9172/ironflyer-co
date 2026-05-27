# Runbook — Rollback

Rollback steps for both the dev path (revert a Go binary) and the
production path (Pulumi-managed Helm).

## Local / dev

```bash
# 1. Identify the last-known-good commit.
git log --oneline -20

# 2. Roll the working tree back.
git checkout <good-commit>

# 3. Rebuild + revet. Both MUST exit 0.
cd core/orchestrator && go build ./... && go vet ./...
cd ../runtime         && go build ./... && go vet ./...

# 4. If migrations were applied since the good commit, you have two
#    choices:
#    a) Stay on the new schema (safe — Go forwards-compatible code
#       tolerates extra columns/tables); restart the older binaries.
#    b) Roll the schema back with the goose CLI:
cd ../orchestrator
POSTGRES_URL="postgres://ironflyer:ironflyer@localhost:5432/ironflyer?sslmode=disable" \
  go run ./cmd/migrate down

# 5. Restart the services.
go run ./cmd/orchestrator
cd ../runtime && go run ./cmd/runtime
cd ../web    && npm run codegen && npm run dev

# 6. Smoke.
bash scripts/v22_smoke.sh
```

The migrate command uses goose under the hood; `migrate up` and
`migrate down` are the only two supported verbs.

## Production (Pulumi-managed Helm)

Fast path — re-pin the image tag:

```bash
cd infra/pulumi
pulumi stack select <stack>
pulumi config set ironflyer:imageTag v0.X.Y    # previous-known-good
pulumi up
kubectl -n ironflyer rollout status deploy/orchestrator
kubectl -n ironflyer rollout status deploy/runtime
kubectl -n ironflyer rollout status deploy/web
```

Stack-history path — revert all Pulumi-managed resources:

```bash
pulumi stack history
pulumi stack export --version <N> > stack-vN.json
pulumi stack import --file stack-vN.json
pulumi up
```

## Data caveat

Managed Postgres and Spaces are stateful. Reverting a Pulumi version
that touches the `data/` slice is **not** automatically reversible.
For schema changes, roll the application back first, then run
`migrate down` against the new-but-failed schema. Point-in-time-
recovery for Postgres and Spaces lives under the broader DR plan —
not in scope for a routine rollback.

## Verification

After rollback:

```bash
curl -s http://localhost:8080/version | jq .   # confirm the older build is back
bash scripts/v22_smoke.sh                       # MUST exit 0 (or PASS-WITH-WARN)
```
