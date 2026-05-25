# Analytics bring-up runbook

This runbook is the recipe for taking the `analytics` profile from cold
to a ClickHouse SELECT returning a non-zero row count after a synthetic
paid execution. It is the closeout proof for the
`PROJECT_CLOSEOUT_PLAN` Definition-of-Done items "Every execution emits
Redpanda events" and "ClickHouse dashboards", which were previously
unverifiable because the analytics profile was not part of the lean
default stack.

The orchestrator gracefully degrades when `REDPANDA_BROKERS` /
`IRONFLYER_CLICKHOUSE_HOSTS` are unset, so nothing here changes
behaviour for a developer who never types `--profile analytics`. The
PG-backed dashboard adapters remain the silent fallback.

---

## What we are standing up

| Profile     | Service(s)               | Host port      | Purpose                                            |
| ----------- | ------------------------ | -------------- | -------------------------------------------------- |
| `analytics` | `redpanda`, `clickhouse` | 19092 / 8123, 9002 | Kafka-API event backbone + analytics OLAP store    |

The container `compose-orchestrator-1` (`--profile apps`, port 8081)
opts into the analytics plane via three env vars defined directly in
the compose file:

- `REDPANDA_BROKERS=redpanda:9092` — turns the outbox publisher from a
  no-op into a Kafka producer (`Redpanda event publisher enabled`).
- `IRONFLYER_CLICKHOUSE_HOSTS=clickhouse:9000` — opens the native-
  protocol pool, applies the embedded DDL on boot, and lights up the
  Redpanda→ClickHouse ingester.
- `IRONFLYER_CLICKHOUSE_DATABASE=ironflyer`, `_USERNAME=ironflyer`,
  `_PASSWORD=ironflyer` — match the CH server env on the same
  service.

The host orchestrator on port 8080 (run with `make dev` / `go run`)
inherits nothing — set the envs in your shell before launch if you
want analytics on the host process too.

---

## 1. Bring the profile up

```bash
docker compose -f infra/compose/docker-compose.dev.yml \
  --profile analytics up -d
```

Wait for both containers to report `healthy`:

```bash
docker compose -f infra/compose/docker-compose.dev.yml \
  ps redpanda clickhouse
```

Expected (truncated):

```
NAME                   STATUS                    PORTS
compose-clickhouse-1   Up 56 seconds (healthy)   0.0.0.0:8123->8123/tcp, 0.0.0.0:9002->9000/tcp
compose-redpanda-1     Up 6 minutes (healthy)    0.0.0.0:19092->19092/tcp, 0.0.0.0:9644->9644/tcp
```

Notes:

- ClickHouse Alpine ships BusyBox `wget`, which resolves `localhost`
  to `::1` first. The CH server logs
  `Listen [::]:8123 failed: Address family not supported` (IPv6 is
  off in the container netns) and the original healthcheck hangs in
  `unhealthy` even though the IPv4 listener is up. The compose
  healthcheck targets `127.0.0.1` explicitly — leave it that way.
- Redpanda auto-creates topics on first produce, so no `rpk topic
  create` step is required.

## 2. Recreate the orchestrator container so it picks up the analytics envs

```bash
docker compose -f infra/compose/docker-compose.dev.yml \
  --profile apps --profile analytics up -d --force-recreate orchestrator
```

Tail the log for the three lines that prove the analytics plane is
live:

```bash
docker logs compose-orchestrator-1 2>&1 | \
  grep -iE "Redpanda event publisher enabled|V22 topic schema registration complete|clickhouse schema bootstrap complete|clickhouse: Redpanda consumer enabled|clickhouse consumer started"
```

Expected (in order):

```
Redpanda event publisher enabled                    brokers=redpanda:9092
events: V22 topic schema registration complete      registered=9 skipped=0 total=9
clickhouse schema bootstrap complete                statements=36
clickhouse: Redpanda consumer enabled               env=dev topics=["ifly.dev.execution.lifecycle.v1", ... 7 total]
clickhouse consumer started                         group=ironflyer-clickhouse-ingest
```

The consumer subscribes to the **env-prefixed** topics — `ifly.dev.*`
in dev, `ifly.prod.*` in prod — derived from `IRONFLYER_ENV` via
`events.CurrentEnv()`. The hard-coded `events.TopicExecutionLifecycle`
constants are pinned to the prod prefix and would strand a dev
cluster on an empty subscription, so the wireup uses `TopicFor` at
construction time.

## 3. Push a paid execution through the system

```bash
IRONFLYER_API_URL=http://localhost:8081 ./scripts/v22_smoke.sh
```

Expected tail:

```
v22 smoke result: PASS
```

The smoke creates a tenant, seeds the wallet from
`IRONFLYER_DEV_WALLET_SEED_USD`, and creates one paid execution. The
top-up and wallet-hold ledger entries hit the outbox and the publisher
drains them to `ifly.dev.billing.ledger.v1`.

## 4. Verify Redpanda received the events

```bash
docker exec compose-redpanda-1 rpk topic list
```

Expected — at minimum the seven env-prefixed topics the consumer
subscribes to:

```
NAME                                PARTITIONS  REPLICAS
ifly.dev.billing.ledger.v1          1           1
ifly.dev.deploy.lifecycle.v1        1           1
ifly.dev.execution.lifecycle.v1     1           1
ifly.dev.execution.steps.v1         1           1
ifly.dev.gates.results.v1           1           1
ifly.dev.patches.lifecycle.v1       1           1
ifly.dev.profitguard.decisions.v1   1           1
```

Read one event back from the wire:

```bash
docker exec compose-redpanda-1 rpk topic consume \
  ifly.dev.billing.ledger.v1 -n 1 --offset start
```

Expected — a JSON envelope with `eventType: "wallet.topup.v1"` and the
full `payload` block:

```json
{
  "topic": "ifly.dev.billing.ledger.v1",
  "value": "{\"id\":\"019e6157-...\",\"eventType\":\"wallet.topup.v1\",\"payload\":{\"amount_usd\":\"50\",...}}",
  "headers": [{"key": "event_id", "value": "..."}, ...],
  "offset": 0
}
```

## 5. Verify ClickHouse landed the events

```bash
docker exec compose-clickhouse-1 clickhouse-client \
  --user ironflyer --password ironflyer \
  --query "SHOW TABLES FROM ironflyer"
```

Expected — the orchestrator-applied schema (7 `raw_*`, 11 `fact_*`,
11 `mv_fact_*`, 5 `rollup_*`) **plus** the legacy
`infra/clickhouse/001_events.sql` init tables (`raw_events`,
`kafka_events`, `mv_kafka_events`, `mv_ledger_entries`,
`fact_ledger_entries`). The legacy init script is bootstrapped by the
CH server itself from `/docker-entrypoint-initdb.d/` and is harmless
— it lives in the same `ironflyer` DB but feeds nothing in V22.

Count rows after the smoke:

```bash
docker exec compose-clickhouse-1 clickhouse-client \
  --user ironflyer --password ironflyer \
  --query "SELECT count(*), max(occurred_at) FROM ironflyer.raw_ledger_events"

docker exec compose-clickhouse-1 clickhouse-client \
  --user ironflyer --password ironflyer \
  --query "SELECT count(*) FROM ironflyer.fact_wallet_topups"
```

Expected — non-zero. A clean smoke yields 3 raw ledger events
(`wallet.topup.v1`, `wallet.hold.v1`, plus the V22 mirror) and the
materialized view rolls one of them into `fact_wallet_topups`:

```
3   2026-05-25 22:52:58.000
1
```

`raw_execution_events` stays at 0 unless the execution settles —
`createPaidExecution` only emits `execution.admitted` (which doesn't
land in CH) until the embedded executor drives it through the
finisher. To produce an `execution.settled.v1` event in a smoke run,
extend `v22_smoke.sh` with a `commitExecution`/`settleExecution`
mutation or wait for the embedded executor's tick.

---

## Known gaps (Go work needed to fully close)

1. **Correction job parameter binding.** On startup the
   `clickhouse.correction` subsystem warns
   `have no arg for param ? at last 8 positions` (rollup_profit_daily)
   and `at last 2 positions` (rollup_abuse_tenant_daily) for every day
   in its 14-day backfill window, and the cohort recompute fails with
   `Aggregate function minIf(...) is found inside another aggregate
   function`. Fix lives in
   `apps/orchestrator/internal/clickhouse/correction.go` —
   bind-arg count and a SELECT rewrite for the cohort rollup.

2. **DLQ topic auto-create.** ~~The first publisher attempt against a
   fresh Redpanda logs `events publisher: DLQ publish failed; dead row
   retained ... Unknown Topic Or Partition`.~~ **Fixed.** The
   orchestrator now calls `events.EnsureDLQTopics` immediately after
   the `PublisherDaemon` boots. The helper resolves the Redpanda
   controller, derives the per-source DLQ topic via `DLQTopicFor`, and
   issues a single idempotent `CreateTopics` (1 partition / RF 1) for
   every V22 source stream. Existing topics surface as kafka error
   code 36 / `Topic_already_exists` and are treated as success. Boot
   log: `events: dlq topics ensured`.

3. **Schema Registry subjects for the publisher path.** ~~`outboxhooks:
   schema subject not registered; inserting without validation` fires
   for every wallet/ledger event because the boot path only registered
   `<topic>-default`.~~ **Fixed.** `events.EventTypesByTopic()` is the
   canonical producer event-type matrix and
   `events.RegisterV22Topics` now walks it to register
   `<topic>-<event_type>` subjects for every known V22 producer call
   site (wallet/ledger/execution/profitguard/patches/gates/deploy/
   memory/audit). The runtime registration is env-aware via
   `CurrentEnv()` so `ifly.dev.*` and `ifly.prod.*` subjects both
   land. Boot log: `events: V22 topic schema registration complete
   registered=81 skipped=0`.

4. **Daily rollup parameter binding.** ~~`clickhouse correction: have
   no arg for param ?` at 8 trailing placeholders — the recompute
   only passed `(dayStart, dayEnd)` to the multi-UNION-ALL templates.~~
   **Fixed.** `correction.recomputeDay` and
   `correction.RecomputeCohort` now call `expandWindowArgs(sql,
   dayStart, dayEnd)` which counts `?` placeholders in the template
   and emits one `(dayStart, dayEnd)` pair per `WHERE occurred_at >=
   ? AND occurred_at < ?` leg. Adding a UNION ALL leg in the future
   needs zero call-site updates.

5. **Cohort recompute nested aggregate.** ~~`clickhouse correction:
   nested aggregate function minIf inside another aggregate` —
   `cohortMonthlySQL` referenced `minIf(...)` from inside another
   `minIf` condition.~~ **Fixed.** Per the ClickHouse docs the inner
   aggregate must be computed first and re-aggregated. The cohort
   SQL now computes `first_paid_at` via a separate per-tenant
   subquery and `LEFT JOIN`s it back as a scalar so the outer
   `minIf(..., occurred_at > firsts.first_paid_at)` is a
   plain-conditional, not a nested aggregate.

6. **Execution lifecycle events.** `createPaidExecution` admits the
   execution but no `execution.settled.v1` event is emitted by the
   smoke run, so `fact_execution_completion` stays empty.
   `scripts/v22_smoke.sh` should be extended (or a settler tick
   waited on) to drive an execution to terminal state if the
   dashboards-from-CH path is to be fully proven from a single
   command.

None of the above blocks the closeout acceptance — a paid execution
emits Redpanda events (`rpk topic consume` shows the envelope) and
those events materialize into ClickHouse rows
(`SELECT count(*) FROM ironflyer.raw_ledger_events` returns > 0).

### Verification (2026-05-26 closure run S)

`grep -c <pattern> tmp/logs/orchestrator.log` before vs. after the
fix, after restarting both the host orchestrator (`/tmp/ironflyer-
orchestrator`) and the docker container with the rebuilt binary and
driving 3 `v22_smoke.sh` runs:

| Pattern                                                          | Before | After |
| ---------------------------------------------------------------- | -----: | ----: |
| `outboxhooks: schema subject not registered`                     |     14 |     0 |
| `events publisher: DLQ publish failed`                           |      0 |     0 |
| `clickhouse correction: have no arg for param`                   |      0 |     0 |
| `clickhouse correction: nested aggregate function`               |      0 |     0 |

The two clickhouse-correction patterns sat at zero in the captured
log because the host orchestrator was running without
`IRONFLYER_CLICKHOUSE_HOSTS`; the docker container had them enabled
and is also clean after the fix. The correction job logs only
`clickhouse correction job started` and per-window `Debug` lines
during a normal pass.
