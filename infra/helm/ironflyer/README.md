# Ironflyer Helm Chart

Production Helm chart for the Ironflyer platform. Installs the
orchestrator, runtime, web, Postgres, and (optionally) the durable event
backbone (Redpanda + Schema Registry), analytics store (ClickHouse),
policy plane (OPA), and demand-driven autoscalers (KEDA).

## Quick Start

```bash
helm install ironflyer ./infra/helm/ironflyer \
  --set postgres.password='<a-strong-random-secret>' \
  --set region=eu \
  --set host=ironflyer.example.com
```

This installs the core control plane only. The durable event backbone,
analytics store, policy plane, and demand-driven autoscalers are all
**opt-in** — see the sections below.

## Full Production Install

```bash
# 1. Install KEDA in-cluster (one-time per cluster).
helm repo add kedacore https://kedacore.github.io/charts
helm install keda kedacore/keda --namespace keda --create-namespace

# 2. Install Ironflyer with the event backbone, analytics, policy plane,
#    and KEDA scalers all turned on.
helm install ironflyer ./infra/helm/ironflyer \
  --set postgres.password='<a-strong-random-secret>' \
  --set region=eu \
  --set host=ironflyer.example.com \
  --set redpanda.enabled=true \
  --set redpanda.schemaRegistry.enabled=true \
  --set clickhouse.enabled=true \
  --set opa.enabled=true \
  --set keda.enabled=true
```

## Optional Components

All four components below are **off by default** and opt-in via their
`enabled` flag. They are designed to be turned on independently as the
platform grows out of single-node development into a multi-tenant
production deploy.

### Redpanda (durable event backbone)

Kafka-compatible event log. Required when the orchestrator's Postgres
outbox publishes durable domain facts (per
`docs/ARCHITECTURE_EVENTS.md`).

```bash
helm upgrade ironflyer ./infra/helm/ironflyer \
  --set redpanda.enabled=true \
  --set redpanda.storageSize=200Gi \
  --set redpanda.replicationFactor=3
```

Notes:
- Single-broker by default (`replicationFactor: 1`). Increase
  `replicationFactor` to scale up the StatefulSet.
- Exposes the Kafka API at `redpanda:9092` (used as KEDA's
  `bootstrapServers` default) and the Schema Registry at
  `redpanda:8081`.
- For multi-broker / production-grade deploys, prefer the upstream
  Redpanda Operator and leave `redpanda.enabled=false`.

#### Redpanda Console (Schema Registry UI)

```bash
helm upgrade ironflyer ./infra/helm/ironflyer \
  --set redpanda.enabled=true \
  --set redpanda.schemaRegistry.enabled=true
```

Console is reachable at `redpanda-console:8080` inside the cluster.

### ClickHouse (analytics store)

Single-replica StatefulSet for Redpanda projection consumers
(dashboards, cost attribution, audit projections per
`docs/ARCHITECTURE_EVENTS.md` § Schema Registry).

```bash
helm upgrade ironflyer ./infra/helm/ironflyer \
  --set clickhouse.enabled=true \
  --set clickhouse.storageSize=100Gi \
  --set clickhouse.storageClass=fast-ssd
```

Notes:
- Reachable at `clickhouse:9000` (native protocol) and `clickhouse:8123`
  (HTTP) inside the cluster.
- For multi-shard production, swap in the official ClickHouse Operator
  chart.

### OPA (Policy Decision Point)

Standalone OPA Deployment + Service for the V22 policy plane
(`docs/ARCHITECTURE_POLICY_SECURITY.md` § OPA / Cedar Decision).

```bash
# Inline default bundles (deny-by-default + tenant isolation).
helm upgrade ironflyer ./infra/helm/ironflyer \
  --set opa.enabled=true

# Production: pull signed bundles from a remote bundle server.
helm upgrade ironflyer ./infra/helm/ironflyer \
  --set opa.enabled=true \
  --set opa.bundleEndpoint=https://policies.ironflyer.com
```

Notes:
- Reachable at `http://opa:8181` from any orchestrator/runtime pod.
- Ships with minimal `default.rego` + `tenant_isolation.rego` ConfigMap;
  point `bundleEndpoint` at a real bundle server for production.
- The V22 target topology is OPA-as-sidecar on every orchestrator pod;
  the standalone Deployment ships today as a working PDP without
  modifying the orchestrator Deployment template. See
  `templates/opa-sidecar.yaml` header for the full compromise note.

### KEDA (event-driven autoscaling)

Two `ScaledObject` triggers that drive replica counts from Redpanda
consumer-group lag instead of raw CPU/HTTP signals (per
`docs/ARCHITECTURE_RUNTIME_SCALE.md` § Kubernetes Scaling).

```bash
# Prerequisite: KEDA installed in-cluster.
helm repo add kedacore https://kedacore.github.io/charts
helm install keda kedacore/keda --namespace keda --create-namespace

# Then enable KEDA in the Ironflyer chart.
helm upgrade ironflyer ./infra/helm/ironflyer \
  --set keda.enabled=true \
  --set keda.kafkaBootstrapServers=redpanda:9092
```

The chart ships two `ScaledObject`s:

| ScaledObject | Target Deployment | Trigger |
| --- | --- | --- |
| `orchestrator-execution-lag` | `orchestrator` | Kafka lag on `ifly.prod.execution.lifecycle.v1` (consumer group `ironflyer-orchestrator`); `lagThreshold=100`, `min=1`, `max=20`. |
| `runtime-allocator-paid-queue` | `runtime` | Kafka lag on `ifly.prod.execution.lifecycle.v1` (consumer group `ironflyer-runtime-allocator`); `lagThreshold=25`, `min=1`, `max=30`. |

The runtime allocator trigger uses the lifecycle topic with the
allocator's distinct consumer group today; V22 is expected to add a
dedicated `paid_queue_depth` Prometheus metric, at which point a second
trigger can layer on without changing the scale target. The trigger is
labelled `ironflyer.io/workerGroupHint=paid-execution-allocator` so
operators can wire metrics dashboards by hint label.

When using a managed Kafka instead of in-cluster Redpanda, set
`keda.kafkaBootstrapServers` to the external endpoint and leave
`redpanda.enabled=false`.

## Default Posture

Out of the box, this chart installs only the core control plane:
orchestrator, runtime, web, Postgres, audit-verify CronJob. All four
optional components above are **off** until explicitly enabled.

## Validation

```bash
# Syntactic + template validation.
helm lint ./infra/helm/ironflyer
helm template ./infra/helm/ironflyer

# With all optional components enabled.
helm template ./infra/helm/ironflyer \
  --set postgres.password=test \
  --set keda.enabled=true \
  --set opa.enabled=true \
  --set clickhouse.enabled=true \
  --set redpanda.enabled=true \
  --set redpanda.schemaRegistry.enabled=true
```

## References

- `docs/ARCHITECTURE_EVENTS.md` — durable event backbone, KEDA scaling
- `docs/ARCHITECTURE_RUNTIME_SCALE.md` — Kubernetes scaling topology
- `docs/ARCHITECTURE_POLICY_SECURITY.md` — OPA / Cedar policy plane
- `docs/DEPLOY.md` — end-to-end production runbook
