# Temporal bring-up runbook

This runbook is the recipe for taking the Temporal profile from cold to
`tctl workflow list` showing a finisher execution that survives an
orchestrator restart. It is the closeout proof for the
`PROJECT_CLOSEOUT_PLAN` Definition-of-Done item
"Temporal can resume interrupted workflows", which was previously
unverifiable because the temporal profile was not part of the lean
default stack.

The embedded executor remains the default for local dev (apps-on-host,
`--profile apps`). Everything below is opt-in: nothing here changes
behaviour for a developer who never types `--profile temporal`.

---

## What we are standing up

Two new opt-in compose profiles, brought up alongside the lean stack
(`postgres + redis + surrealdb + minio`) and the apps already running
either on host or under `--profile apps`:

| Profile          | Service(s)                | Host port | Purpose                                       |
| ---------------- | ------------------------- | --------- | --------------------------------------------- |
| `temporal`       | `temporal`, `temporal-ui` | 7233/8233 | Temporal frontend + UI, schema-backed by `postgres` |
| `temporal-orch`  | `orchestrator-temporal`   | 8082      | Second orchestrator in `IRONFLYER_EXECUTOR=temporal` mode, registers a worker on the `ironflyer-finisher` task queue |

The embedded `compose-orchestrator-1` (`--profile apps`, port 8081) and
the host orchestrator (port 8080) keep running unchanged. Only
`orchestrator-temporal` connects to Temporal.

---

## 1. Bring up the Temporal profile

```bash
docker compose -f infra/compose/docker-compose.dev.yml --profile temporal up -d
```

Schema bootstrap into `postgres` takes ~30-60 s on first run. Confirm
both containers are up:

```bash
docker compose -f infra/compose/docker-compose.dev.yml ps temporal temporal-ui
```

Expect both at `Up`. The frontend serves on `localhost:7233`, the UI on
`http://localhost:8233`.

Health-check the cluster directly:

```bash
docker exec compose-temporal-1 tctl --address temporal:7233 cluster health
# -> temporal.api.workflowservice.v1.WorkflowService: SERVING
```

### Gotcha: `DYNAMIC_CONFIG_FILE_PATH`

The `temporalio/auto-setup:1.25` image does NOT ship the
`config/dynamicconfig/development-sql.yaml` file the original env-var
pointed at. With that env unset, the server crashes on boot with:

```
Unable to create dynamic config client. Error: unable to validate
dynamic config: dynamic config: config/dynamicconfig/development-sql.yaml:
no such file or directory
```

Compose now mounts an empty override file from
`infra/compose/temporal/dynamicconfig/development-sql.yaml` to
`/etc/temporal/config/dynamicconfig/development-sql.yaml`, so server
defaults apply. Add per-namespace knobs there only if a workflow
actively needs to override the dev defaults.

---

## 2. Stand up the temporal-mode orchestrator

```bash
docker compose -f infra/compose/docker-compose.dev.yml \
  --profile temporal --profile temporal-orch up -d
```

`orchestrator-temporal` runs with:

- `IRONFLYER_EXECUTOR=temporal`
- `IRONFLYER_TEMPORAL_HOST=temporal:7233`
- `IRONFLYER_TEMPORAL_NAMESPACE=default`
- `IRONFLYER_TEMPORAL_TASK_QUEUE=ironflyer-finisher`
- Same Postgres + dev wallet seed env as the embedded orchestrator so
  `scripts/v22_smoke.sh` runs unmodified against it.

Host port defaults to `8082` (override with `ORCHESTRATOR_TEMPORAL_HOST_PORT`).

Confirm the worker started:

```bash
docker compose -f infra/compose/docker-compose.dev.yml \
  logs orchestrator-temporal | grep "Temporal finisher worker started"
# -> Temporal finisher worker started host=temporal:7233
#    namespace=default svc=orchestrator task_queue=ironflyer-finisher
```

If you see `Temporal worker disabled; embedded finisher executor active`
the env didn't reach the container — re-check `IRONFLYER_EXECUTOR` and
`IRONFLYER_TEMPORAL_HOST`.

---

## 3. Smoke against :8082

```bash
IRONFLYER_API_URL=http://localhost:8082 bash scripts/v22_smoke.sh
```

The script signs up a fresh user, asserts wallet seed, creates a
`static-landing` paid execution, opens an `executionFeed`
subscription, then reads `wallet / ledger / execution / profitDashboard`.

Expected tail:

```
v22 smoke result: PASS
```

PASS proves the temporal-mode orchestrator answers GraphQL, lands the
wallet hold, admits the execution, and refreshes the row. Note that
`createPaidExecution` itself does NOT yet dispatch
`FinisherExecutionWorkflow` — see "Known Go-level gap" below — so the
finisher work still runs through the embedded engine fire-and-forget
goroutine.

---

## 4. Prove the worker actually processes Temporal tasks

Because the resolver does not yet call `client.ExecuteWorkflow`, we
exercise the worker directly via `tctl`:

```bash
docker exec compose-temporal-1 tctl --address temporal:7233 -ns default \
  workflow start \
  --taskqueue ironflyer-finisher \
  --workflow_type FinisherExecutionWorkflow \
  --workflow_id smoke-<exec-uuid> \
  --input '{"ExecutionID":"<exec-uuid>","TenantID":"<tenant-uuid>","BudgetUSD":"1","BlueprintID":"static-landing"}' \
  --execution_timeout 600
```

Confirm Temporal sees the workflow:

```bash
docker exec compose-temporal-1 tctl --address temporal:7233 -ns default workflow list
# -> FinisherExecutionWorkflow | smoke-... | ironflyer-finisher | ...
```

Same view is available in the Temporal UI at
[http://localhost:8233](http://localhost:8233). For the closeout proof
run the UI showed a `FinisherExecutionWorkflow` row on the
`ironflyer-finisher` task queue with a populated history.

---

## 5. Resume test

Goal: confirm Temporal hands a queued workflow to the worker after the
worker container has been killed and brought back.

```bash
# 5a. Stop the worker.
docker compose -f infra/compose/docker-compose.dev.yml stop orchestrator-temporal

# 5b. Start a workflow while no worker is polling.
docker exec compose-temporal-1 tctl --address temporal:7233 -ns default \
  workflow start \
  --taskqueue ironflyer-finisher \
  --workflow_type FinisherExecutionWorkflow \
  --workflow_id resume-test-$(date +%s) \
  --input '{"ExecutionID":"resume-test","TenantID":"resume-test","BudgetUSD":"1","BlueprintID":"static-landing"}' \
  --execution_timeout 600

# 5c. Confirm it sits in Running with history length 2 — i.e. the
# WorkflowExecutionStarted event landed but no worker has touched it.
docker exec compose-temporal-1 tctl --address temporal:7233 -ns default \
  workflow describe -w resume-test-<ts>

# 5d. Bring the worker back.
docker compose -f infra/compose/docker-compose.dev.yml \
  --profile temporal --profile temporal-orch start orchestrator-temporal

# 5e. After ~10 s the same workflow has advanced (history length 5+
# and status moved to Completed / Failed).
docker exec compose-temporal-1 tctl --address temporal:7233 -ns default \
  workflow describe -w resume-test-<ts>
```

In the closeout proof run the workflow went from
`status=Running, historyLength=2` (worker down) to
`status=Failed, historyLength=5, closeTime=...` (worker back).
The `Failed` terminal here is the expected outcome for the synthetic
input — `resume-test` is not a real execution row, so the AdmitExecution
activity rejects it. The point is that the worker polled the task,
processed it, and Temporal closed the run — i.e. resume works.

To exercise resume on a real execution, mid-flight, kill the worker
container while a long workflow is between activities; Temporal will
re-dispatch the next activity to the worker when it reconnects.

---

## 6. Bringing it all down

```bash
# Stop just the temporal-mode orchestrator (keep Temporal running):
docker compose -f infra/compose/docker-compose.dev.yml stop orchestrator-temporal

# Stop the whole temporal profile:
docker compose -f infra/compose/docker-compose.dev.yml \
  --profile temporal --profile temporal-orch down

# Nuke Temporal schema (forces auto-setup to re-bootstrap on next up):
docker compose -f infra/compose/docker-compose.dev.yml exec postgres \
  psql -U ironflyer -c "DROP DATABASE IF EXISTS temporal;"
docker compose -f infra/compose/docker-compose.dev.yml exec postgres \
  psql -U ironflyer -c "DROP DATABASE IF EXISTS temporal_visibility;"
```

---

## Known Go-level gap

`core/orchestrator/internal/operations/graph/resolver/execution.resolver.go` →
`CreatePaidExecution` fires the embedded `*finisher.Engine.Run` in a
goroutine and never calls `client.ExecuteWorkflow` against Temporal,
even when `IRONFLYER_EXECUTOR=temporal`. The Temporal worker is
correctly registered (see step 4), but no production code path
dispatches to it on its own — only `tctl` does today.

Closing that gap is a wireup change in the resolver / a new
`temporalDispatcher` adapter, which is out of scope for this runbook
(the closeout plan explicitly forbids redesigning the worker here).
File the gap as "wire `CreatePaidExecution` to `client.ExecuteWorkflow`
when `IRONFLYER_EXECUTOR=temporal`" — pure wireup, no new gates or
activities required.
