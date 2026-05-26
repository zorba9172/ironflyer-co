# Ironflyer Event Backbone

This document defines the durable event backbone for Ironflyer. It is
deliberately separate from the existing Redis bus: Redis remains the
low-latency, ephemeral fan-out layer for GraphQL subscriptions, SSE,
presence, cursors, PTY output, locks, rate limits, and short-lived state.
Redpanda is the durable asynchronous backbone for facts that must survive
pod restarts, drive independent consumers, be replayed, or scale with
consumer lag.

## Decision

Ironflyer publishes committed domain facts through a Postgres outbox into
Redpanda topics. Consumers process those topics with idempotent writes to
Postgres projections, object storage, search/vector indexes, webhooks,
analytics stores, and external integrations.

```
transactional writer
  -> Postgres business tables
  -> Postgres event_outbox row
  -> outbox publisher
  -> Redpanda topic
  -> consumer group
  -> idempotent side effect
```

Postgres is the source of truth. Redpanda carries committed facts and
replayable work, not primary state.

## Redis Boundary

Do not dual-publish application events directly to both Redis and
Redpanda. A durable fact is written once to the Postgres outbox, then
published to Redpanda. Live UI surfaces use the existing Redis/in-process
bus for fan-out and Postgres as the resume source.

| Need | System | Reason |
| --- | --- | --- |
| GraphQL subscription fan-out across pods | Redis bus | Low latency, recoverable from persisted state |
| Presence, cursors, PTY, inline completions | Redis or in-process bus | Ephemeral and connection-scoped |
| Locks, rate limits, short TTL coordination | Redis | Atomic TTL primitives |
| Durable cross-service event delivery | Redpanda | Retention, consumer groups, replay |
| Projection rebuilds and delayed catch-up | Redpanda | Offset management and ordered replay |
| Source of truth | Postgres | Transactional invariants and economic correctness |

## What Belongs In Redpanda

- Durable domain facts used by more than one component.
- Work that may be retried independently of the request that created it.
- Events that power analytics, audit projections, dashboards, webhooks,
  billing exports, search indexing, and integrations.
- High-volume append-only streams where ordering matters inside a tenant,
  execution, project, deploy, or workspace key.
- Events that may need replay after a bug fix, projection rebuild, or
  downstream outage.

## What Does Not Belong In Redpanda

- Request/response RPC, GraphQL resolver hops, or synchronous admission
  decisions. Use direct service calls or Postgres transactions.
- Distributed locks, rate limits, leases, session revocation cache, and
  ephemeral coordination. Keep these in Redis.
- Live UI fan-out where dropped payloads can be recovered from persisted
  state. Keep GraphQL subscriptions and SSE on the Redis/in-process bus.
- Presence, cursors, keystrokes, PTY byte streams, and websocket-only
  transient state.
- Large blobs, patches, snapshots, exports, logs, or model artifacts.
  Store payloads in S3/R2/MinIO and publish a small event with a URI,
  checksum, byte count, and retention class.
- Secrets, provider credentials, raw payment data, full prompts containing
  sensitive user content, or unredacted webhook payloads.
- Mutable state snapshots that consumers are expected to overwrite as
  truth. Publish facts; build projections from them.

## Outbox Contract

Every service that creates a durable event writes it inside the same
Postgres transaction as the state change it describes.

Minimum `event_outbox` fields:

| Column | Purpose |
| --- | --- |
| `id` | Globally unique event id, preferably UUIDv7 or ULID |
| `aggregate_type` | Domain owner: `execution`, `wallet`, `ledger`, `deploy`, etc. |
| `aggregate_id` | Stable entity id used for partition routing |
| `event_type` | Fully qualified type, e.g. `execution.step_completed.v1` |
| `schema_subject` | Schema Registry subject |
| `schema_version` | Writer schema version |
| `payload` | JSONB payload matching the registered schema |
| `headers` | JSONB metadata: tenant, trace, causation, correlation |
| `occurred_at` | Domain event time |
| `available_at` | Delayed publish time, normally equal to `occurred_at` |
| `published_at` | Set after Redpanda ack |
| `publish_attempts` | Publisher retry counter |
| `last_publish_error` | Truncated diagnostic text |

Required headers:

- `event_id`: same as outbox `id`.
- `tenant_id`: tenant boundary for authorization and replay scopes.
- `correlation_id`: user request, workflow, or command that caused the
  event.
- `causation_id`: upstream event id when produced by a consumer.
- `traceparent`: OpenTelemetry propagation.
- `producer`: service name and version.
- `idempotency_key`: stable key for consumer deduplication.

The publisher leases unpublished rows with `FOR UPDATE SKIP LOCKED`,
publishes in bounded batches, waits for Redpanda acks, then marks
`published_at`. If Redpanda is unavailable, writers still commit the
database transaction and the publisher catches up later.

## Topic Taxonomy

Topic names are environment-prefixed and domain-oriented:

```
ifly.<env>.<domain>.<stream>.v<major>
```

| Topic | Key | Producers | Consumers |
| --- | --- | --- | --- |
| `ifly.prod.execution.lifecycle.v1` | `execution_id` | Orchestrator | dashboards, audit projections, notifications |
| `ifly.prod.execution.steps.v1` | `execution_id` | Finisher engine | completion scoring, cost attribution, analytics |
| `ifly.prod.gates.results.v1` | `project_id` | Gate runners | repair planner, quality dashboards |
| `ifly.prod.patches.lifecycle.v1` | `project_id` | Patch lifecycle | patch memory, audit, notifications |
| `ifly.prod.billing.ledger.v1` | `tenant_id` | Wallet/ledger | profit dashboards, exports, finance reconciliation |
| `ifly.prod.profitguard.decisions.v1` | `execution_id` | ProfitGuard | dashboards, policy analysis, alerts |
| `ifly.prod.deploy.lifecycle.v1` | `deploy_id` | Deploy engine | deploy projections, notifications, domain automation |
| `ifly.prod.webhooks.delivery.v1` | `webhook_delivery_id` | Webhook dispatcher | retry workers, tenant delivery analytics |
| `ifly.prod.integrations.github.v1` | `project_id` | Integration adapters | sync workers, audit projections |
| `ifly.prod.memory.indexing.v1` | `project_id` | Patch/spec writers | memory/search indexers |
| `ifly.prod.audit.security.v1` | `tenant_id` | Auth/security gates | SIEM export, compliance reports |

Partition by the narrowest aggregate that needs in-order processing. Use
`execution_id` for execution step order, `tenant_id` for wallet/ledger
order, `project_id` for patch and memory order, and `deploy_id` for
deploy order. Never key by random event id when consumers need
per-aggregate order.

Topic major versions change only for incompatible schema or semantic
breaks. Compatible schema evolution remains in the same topic through
Schema Registry.

## Schema Registry

All Redpanda topics use Schema Registry with one subject per event type:

```
<topic>-<event_type>
```

Schemas are JSON Schema by default because Ironflyer already uses typed
JSON shapes across agents, GraphQL-adjacent payloads, and audit records.
Avro or Protobuf can be introduced for very high-throughput streams only
after the owning service provides generated types and compatibility
tests.

Compatibility rules:

- Default mode: backward compatible.
- Additive fields must be optional or have defaults.
- Removing, renaming, or changing the meaning of a field requires a new
  event type version and may require a new topic major version.
- Consumers must ignore unknown fields.
- Producers must validate payloads before inserting into the outbox.
- CI should register schemas in a staging registry and reject
  incompatible changes before merge.

Each event has a stable envelope plus a typed payload. The envelope is
shared infrastructure; the payload is owned by the producing domain.

## Delivery, Idempotency, And Ordering

Redpanda delivery is at-least-once. Consumers must be idempotent.

Consumer requirements:

- Record processed `event_id` or `idempotency_key` in the same
  transaction as the side effect.
- Use unique constraints for externally visible effects, e.g.
  `(consumer_name, event_id)` or a domain-specific natural key.
- Commit offsets only after the side effect commits.
- Treat duplicate events as success.
- Preserve order only within a partition key; never assume global order
  across a topic.
- Use bounded concurrency per partition and independent concurrency
  across partitions.

Producer requirements:

- One outbox row per durable fact.
- Deterministic `event_id` only when retrying the same logical write;
  otherwise unique ids.
- No direct consumer side effects in the request transaction beyond the
  source-of-truth write and outbox insert.
- No direct Redpanda publish from request handlers unless the event has
  already been committed to the outbox.

## Dead Letter Queues

Each consumer group has a matching DLQ topic:

```
ifly.<env>.dlq.<source-domain>.<source-stream>.<consumer>.v1
```

DLQ records include original topic, partition, offset, key, headers,
event id, consumer name/version, failure class, attempt count, final
error summary, first failure time, and DLQ time.

Policy:

- Transient failures retry with exponential backoff and jitter before
  DLQ.
- Schema and validation failures DLQ quickly because retrying them blocks
  the partition without progress.
- DLQs are observable and page only when the event class affects money,
  security, deploy correctness, or user-visible workflow completion.
- DLQ replay requires an operator action with a reason, target consumer
  group, and optional schema/transform version.

## Replay

Replay is an explicit operational workflow, not an accidental side effect
of redeploying a service.

Supported replay modes:

- Offset replay: reset a consumer group to a timestamp or offset range.
- Projection rebuild: start a new consumer group and rebuild a table or
  index into a shadow projection, then swap.
- DLQ replay: read a DLQ topic, optionally transform, then re-submit to
  the original topic or a repair topic.
- Tenant or aggregate replay: filter by `tenant_id`, `aggregate_type`,
  and `aggregate_id` when rebuilding scoped state.

Replay safety rules:

- Replayed consumers must use the same idempotency table or a scoped
  replay idempotency namespace.
- External side effects such as emails, webhooks, deploy mutations,
  refunds, and provider calls are disabled by default during replay
  unless the replay command explicitly enables them.
- Economic projections can be rebuilt from ledger facts, but ledger
  source-of-truth rows are never rewritten by replay.
- Replay jobs emit audit events and expose progress metrics.

## KEDA Scaling

Consumers scale from Redpanda lag through KEDA Kafka scalers.

Standard scaler inputs:

- `bootstrapServers`: Redpanda brokers.
- `consumerGroup`: the deployment's consumer group.
- `topic`: one topic per scaler trigger when lag profiles differ.
- `lagThreshold`: messages per replica target.
- `activationLagThreshold`: minimum lag before scaling above zero.
- `offsetResetPolicy`: normally `latest` for new online workers, and
  `earliest` only for replay or projection rebuild workers.

Guidance:

- Use separate Deployments and consumer groups for online processing,
  exports, and replay jobs so replay does not starve live work.
- Cap max replicas by partition count unless the consumer does useful
  work outside partition order.
- Alert on lag age, not just lag count, for low-volume money/security
  topics.
- Expose `consumer_lag`, `oldest_unprocessed_event_age_seconds`,
  `events_processed_total`, `event_process_duration_seconds`,
  `event_retries_total`, and `event_dlq_total`.

## Retention

| Topic class | Suggested retention |
| --- | --- |
| Ledger, ProfitGuard, audit/security facts | 1 year or compliance policy |
| Execution lifecycle, gates, patches, deploys | 30-90 days |
| Analytics/indexing work queues | 7-30 days |
| DLQ topics | 30-90 days, depending on severity |
| Replay repair topics | Short-lived, deleted after completion |

Postgres remains the long-term source of truth for economic and product
state. Object storage owns large artifacts and snapshots.

## Security And Privacy

- Events carry `tenant_id`; consumers enforce tenant isolation even when
  topics are shared.
- Sensitive fields are redacted or tokenized before outbox insertion.
- Topic ACLs are per service account and least privilege.
- Schema Registry writes are restricted to CI and owning service deploys.
- Consumer logs never print full payloads by default; log event id, event
  type, topic, partition, offset, tenant id, and failure class.
- Cross-region replication must respect tenant data residency policy.

## Rollout Plan

1. Add the `event_outbox` table, publisher lease loop, and publisher
   metrics.
2. Introduce Schema Registry and register the envelope plus the first
   domain schemas.
3. Publish `billing.ledger`, `execution.lifecycle`, and
   `profitguard.decisions` events from existing Postgres transactions.
4. Build one low-risk projection consumer and validate idempotency, DLQ,
   replay, and KEDA lag scaling.
5. Move durable integration work to Redpanda consumer groups while
   keeping Redis for live fan-out.
6. Add replay runbooks and restrict operator replay permissions.

## Invariants

- Postgres owns correctness; Redpanda owns durable asynchronous delivery.
- Redis remains ephemeral and low-latency; Redpanda remains replayable
  and at-least-once.
- Every durable event starts in the outbox.
- Every consumer is idempotent.
- Every schema is registered and compatibility-checked.
- Every DLQ event is observable and replayable by an operator.
- No large blobs, secrets, or live websocket byte streams are placed in
  Redpanda.

## Current state (2026-05-26 audit)

End-to-end outbox path verified:

- Writer: `outboxhooks.WriteEventInTx` writes the row inside the
  caller's `pgx.Tx`, validating topic + schema before insert
  (`core/orchestrator/internal/business/outboxhooks/outboxhooks.go`).
- Claim: `PostgresOutbox.Claim` uses
  `FOR UPDATE SKIP LOCKED` with a worker lease, incrementing attempts
  atomically (`internal/operations/events/postgres.go`).
- Publish: `PublisherDaemon.drainOnce` calls `pub.Publish` first, then
  `MarkPublished` only after the broker ack; failures schedule
  exponential backoff via `MarkFailed`
  (`internal/operations/events/publisher.go`). RequiredAcks=All on
  the Kafka writer (`redpanda.go`).
- DLQ: per-source DLQ topics are pre-created at boot via
  `EnsureDLQTopics`; exhausted retries emit one `dlq.v1` record and
  the dead row is retained on `mark_failed dead=true`.
- Consumer: ClickHouse consumer reads from the configured topic set
  and commits offsets only after INSERT success
  (`internal/business/clickhouse/consumer.go`). Idempotency is
  ReplacingMergeTree dedup keyed on `event_id`.
- MemoryGraph: writer is registered as an Observer on the publisher
  daemon and projects every successfully-published event into the
  knowledge graph (`internal/operations/wireup/memorygraph.go`).
