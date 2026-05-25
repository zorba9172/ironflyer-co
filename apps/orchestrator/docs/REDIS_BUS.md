# Cross-pod event bus (Redis pub/sub backbone)

The orchestrator's GraphQL subscriptions and SSE streams are powered by an
in-process event fan-out. When the orchestrator runs as more than one pod
behind a load balancer, a subscriber connected to pod A must also see
events fired on pod B — otherwise horizontal scale silently breaks every
live surface (run streams, chat, cost ticks, deploys, presence, cursors,
collab chat, figma imports, inline completions, PTY output).

This document describes the bus that closes that gap.

## Components

- `internal/bus/Bus` — the interface every backend implements
  (`Publish`, `Subscribe`, `Close`).
- `internal/bus/MemoryBus` — single-pod backend, used when no Redis is
  configured. `Publish` walks an in-process subscriber map; `Subscribe`
  hands back a buffered channel.
- `internal/bus/RedisBus` — Redis pub/sub backend, wraps the existing
  go-redis client already used for distributed locks + rate limits.
  Channel names are namespaced as `ironflyer:bus:<topic>`.
- `internal/bus/Multiplexer` — fronts whichever backend is selected. It
  owns the local subscriber map so same-pod delivery never round-trips
  through Redis, and it stamps every cross-pod payload with an 8-byte
  pod ID so a pod can drop the echo of its own publish (the dedup
  mechanism — see below).

## Backend selection

`cmd/orchestrator/main.go` constructs the bus during boot:

- `IRONFLYER_REDIS_ENABLED=true` plus a reachable Redis → `RedisBus`.
  Events cross pods.
- Otherwise → `MemoryBus`. Behaviour is identical to the pre-bus single
  pod path.

Logged loudly at startup (`backend=redis` or `backend=memory`).

## Topic taxonomy

Topics use colon-delimited segments. The first segment is the **kind**
(bounded cardinality for metrics labels); the trailing segments are the
keys (project ID, user ID, etc.) that scope the broadcast.

| Kind                  | Topic pattern                                | Producer                                    | Consumer                              |
| --------------------- | -------------------------------------------- | ------------------------------------------- | ------------------------------------- |
| `finisher_run`        | `finisher.run:<projectID>`                   | `internal/finisher.Engine`                  | `Subscription.runProject`             |
| `cost`                | `cost:<userID>`                              | `internal/providers.MemorySink`             | `Subscription.costStream`             |
| `deploy`              | `deploy:<deployID>`                          | `internal/httpapi.DeployEngine`             | `Subscription.deployStream`           |
| `collab_presence`     | `collab.presence:<projectID>`                | `internal/collab.Hub`                       | `Subscription.collabPresence`         |
| `collab_cursors`      | `collab.cursors:<projectID>`                 | `internal/collab.Hub`                       | `Subscription.collabCursors`          |
| `collab_chat`         | `collab.chat:<projectID>`                    | `internal/collab.Hub`                       | `Subscription.collabChat`             |
| `figma`               | `figma:<importID>`                           | `internal/figma.Publisher`                  | `Subscription.figmaImportStatus`      |
| `inline`              | `local:inline:<userID>:<requestID>`          | `internal/httpapi` inline handler           | `Subscription.inlineCompletion`       |
| `workspace`           | `workspace:<workspaceID>:pty`                | `internal/httpapi` workspace PTY handler    | `Subscription.workspacePty`           |

Notes:

- The `local:` prefix on the inline topic marks it as same-pod only —
  the Multiplexer never publishes those across pods because the inline
  completion producer and consumer are always the same request handler.
- Workspace PTY topics are not yet bus-bridged in code (the resolver is
  currently `not implemented`); the topic is reserved for the Round 10
  workspace runtime integration.

## Dedup mechanism

The Multiplexer publishes an 8-byte pod ID prefix on every cross-pod
message. When the reader goroutine receives a message via the backing
Bus and finds its own pod ID as the prefix, it drops the message — the
local fan-out path already delivered it. This is the canonical dedup.

Resolvers that consume long-lived event streams (e.g. `runProject`)
layer an additional **event-ID** dedup on top: every `domain.Event`
carries a `evt-<timestamp>-<counter>` ID and the resolver keeps a small
FIFO ring of recently-seen IDs. This protects against the rare case
where the producer flushes the same event to both its in-process
subscriber map *and* the bus and the consumer happens to read both
before the dedup window closes.

## Routing

Because every event reaches every pod, the Kubernetes Service can use
the default round-robin load balancer. **Sticky sessions are not
required** for any GraphQL subscription that goes through the bus.

## Metrics

Registered in `internal/metrics/metrics.go`:

- `bus_messages_published_total{topic_kind}` — count of publishes.
- `bus_messages_received_total{topic_kind,source}` — count of deliveries
  to local subscribers; `source` is `local` (publish on this pod) or
  `remote` (message arrived from another pod via Redis).
- `bus_subscriber_drop_total{topic_kind}` — bumped whenever a subscriber
  channel was full and we dropped the payload rather than block the
  producer (slow consumer).
- `bus_active_subscribers{topic_kind}` — current count of in-process
  subscribers per topic kind.

`topic_kind` is the first colon-segment of the topic, with dots replaced
by underscores so `collab.presence` becomes `collab_presence`. Cardinality
is bounded by the table above.

## Resilience

- **Reconnect.** RedisBus uses go-redis's `Channel()` reader, which
  auto-reconnects on transient transport errors. Subscribers do not
  need to do anything special; the reader loop continues from the next
  message after the reconnect.
- **Slow consumers.** Each subscriber channel is buffered at
  `bus.SubBuffer` (256). When the buffer fills, the Multiplexer drops
  the message and bumps `bus_subscriber_drop_total`. SSE clients
  reconnect on `RST_STREAM` and resume from the project's persisted
  event log — no on-the-wire event is load-bearing.
- **Publish failure fallback.** A producer that hits a Redis transport
  error still delivers locally (the in-process fan-out runs first). The
  Multiplexer logs the error; the producer does not retry — the event
  is on the in-process subscriber's channel already.

## Development vs production

- **Single-pod dev** (`IRONFLYER_REDIS_ENABLED` unset): the orchestrator
  uses `MemoryBus`. Every subscription works exactly as it did before
  the bus was introduced.
- **Multi-pod production**: set `IRONFLYER_REDIS_ENABLED=true` and point
  `IRONFLYER_REDIS_ADDR` at the cluster's Redis. Subscribers on pod A
  see events fired on pod B with sub-millisecond latency on a healthy
  Redis link.
