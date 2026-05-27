# `infra/k8s/` — deprecated as the production path

**Status (2026-05-27):** kept in-tree for reference only.

The production stack is now defined in
[`infra/compose/docker-compose.prod.yml`](../compose/docker-compose.prod.yml).
That Compose file is the single source of truth and is what
[`DEPLOY.md`](../../DEPLOY.md) installs.

The Kustomize bundles here are kept because some operators prefer
plain manifests over Helm — see [`infra/helm/DEPRECATED.md`](../helm/DEPRECATED.md)
for the same notes that apply to this directory.
