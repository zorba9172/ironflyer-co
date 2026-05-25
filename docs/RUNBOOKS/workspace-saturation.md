# Runbook — Workspace saturation

The runtime owns per-user sandboxes. When the sandbox driver runs out
of capacity, paid executions queue, ProfitGuard pauses, and the
customer experience degrades — sometimes silently.

## Symptoms

- `executionFeed` subscription emits long `waiting_for_workspace` gaps.
- `createPaidExecution` succeeds but `status` stays at `admitted` for
  far longer than the blueprint's normal allocation latency.
- Runtime `/healthz` is OK but `services.driver=docker` and `docker
  ps | wc -l` is near the configured ceiling.

## 1. Confirm saturation

```bash
# Runtime self-report.
curl -s http://localhost:8090/healthz | jq .

# Docker-driver only: count live workspace containers.
docker ps --filter 'label=ironflyer.workspace=true' --format '{{.ID}}' | wc -l
```

If the count is at or above `IRONFLYER_RUNTIME_MAX_WORKSPACES` (or the
driver-specific equivalent), saturation is real.

## 2. Drain idle workspaces

The runtime maintains a per-workspace last-touch timestamp. Operators
can prune via the runtime admin surface — or, in dev, by stopping
containers directly:

```bash
# Prune any workspace container idle for > 15 minutes.
docker ps --filter 'label=ironflyer.workspace=true' \
          --format '{{.ID}} {{.Status}}' \
  | awk '$2 ~ /minutes/ && $3+0 > 15 { print $1 }' \
  | xargs -r docker stop
```

## 3. Add capacity

- **Mock driver dev path** — switch the runtime to the Docker driver
  for headroom:
  ```bash
  IRONFLYER_RUNTIME_DRIVER=docker go run ./cmd/runtime
  ```
- **Single-node prod** — scale `replicas` on the runtime Deployment:
  ```bash
  kubectl -n ironflyer scale deploy/runtime --replicas=<n>
  ```
- **Multi-node prod** — bump the EKS / DOKS node group size via Pulumi
  and reapply.

## 4. Stop admitting new paid executions until headroom returns

If saturation is the bottleneck and not provider cost, the ProfitGuard
verdict you want is `pause`, not `stop`. Setting
`IRONFLYER_PROFITGUARD_DEFAULT=pause` and restarting the orchestrator
leaves existing executions running while refusing new admits.

## 5. Verification

```bash
# Counts back below ceiling.
docker ps --filter 'label=ironflyer.workspace=true' --format '{{.ID}}' | wc -l

# v22_smoke creates one paid execution; it must still admit and the
# wallet hold must land. If it does not, capacity is still wrong.
bash scripts/v22_smoke.sh
```
