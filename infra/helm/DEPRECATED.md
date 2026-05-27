# `infra/helm/` — deprecated as the production path

**Status (2026-05-27):** kept in-tree for reference only.

The production stack is now defined in
[`infra/compose/docker-compose.prod.yml`](../compose/docker-compose.prod.yml).
That Compose file is the single source of truth and is what
[`DEPLOY.md`](../../DEPLOY.md) installs.

## When to use the Helm chart anyway

- You already operate a Kubernetes cluster (DOKS, EKS, GKE, on-prem
  k8s, or k3s) and don't want to introduce a second deployment shape
  alongside it.
- You need every-pod-on-its-own-node placement at a scale the single
  AX102 + warm-standby pair can't carry. In practice that's
  >50 concurrent paid users — well past the budget envelope the
  Compose stack is sized for.

## What stays in sync

When you change service env vars, image tags, or feature flags, update
**both** the Compose file and the Helm `values-prod.yaml` until this
directory is removed. The Compose file is authoritative; Helm follows.

## What does NOT stay in sync

- `kube-prometheus-stack` (Helm) → `victoriametrics` + `vmagent` (Compose)
- `loki-stack` distributed (Helm) → `loki` single-binary (Compose)
- `ingress-nginx` + `cert-manager` + `external-dns` + `sealed-secrets`
  (Helm) → `caddy` (Compose)
- Managed Postgres / Valkey / Spaces (Helm assumes DO managed) → in-cluster
  `postgres` + `redis` + `minio` (Compose)
- `keda` + `opa` + `audit-verify` (Helm) → handled by the orchestrator
  itself in the Compose path (queue-depth-driven sandbox spawn, Go-side
  authorization middleware, scripted audit cron)

If you need any of those k8s-specific capabilities, the Helm chart is
where they live.
