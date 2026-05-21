# Ironflyer — Kubernetes manifests

Minimal, opinionated layout for deploying the three Ironflyer services
(`orchestrator`, `runtime`, `web`) plus a single Postgres StatefulSet to any
conformant Kubernetes cluster.

What you get:

| Component | Service | Port | Replicas |
| --- | --- | --- | --- |
| orchestrator | `orchestrator.ironflyer.svc` | 8080 | 2 |
| runtime | `runtime.ironflyer.svc` | 8090 | 1 (workspace state is per-pod) |
| web | `web.ironflyer.svc` | 3000 | 2 |
| postgres | `postgres.ironflyer.svc` (headless) | 5432 | 1 (StatefulSet) |
| ingress | `ironflyer.example.com` | 80/443 | n/a |

External dependencies the cluster operator brings:

- Container images at `ghcr.io/zorba9172/ironflyer-{orchestrator,runtime,web}` —
  build with the Dockerfiles in `infra/docker/` and push to your registry.
- An ingress controller (defaults to `ingressClassName: nginx`).
- A TLS secret named `ironflyer-tls` for `ironflyer.example.com`.
- For production, replace the in-cluster Postgres with managed Postgres and
  point `orchestrator-config.POSTGRES_URL` at it.
- Temporal / SurrealDB / MinIO are **not** in this bundle — the orchestrator
  defaults to `executor=embedded` + `db=memory` so the cluster boots without
  them. Add them when you need the corresponding feature.

## Apply

```bash
# 1. Build + push images (replace registry).
docker build -f infra/docker/orchestrator.Dockerfile -t ghcr.io/zorba9172/ironflyer-orchestrator:v0.1 .
docker build -f infra/docker/runtime.Dockerfile -t ghcr.io/zorba9172/ironflyer-runtime:v0.1 .
docker build -f infra/docker/web.Dockerfile -t ghcr.io/zorba9172/ironflyer-web:v0.1 .

# 2. Copy + fill the secret template; do NOT commit the filled file.
cp infra/k8s/orchestrator/secret.example.yaml infra/k8s/orchestrator/secret.yaml
$EDITOR infra/k8s/orchestrator/secret.yaml
kubectl apply -f infra/k8s/orchestrator/secret.yaml

# 3. Apply everything else via kustomize.
kubectl apply -k infra/k8s
```

## Notes

- The runtime uses the same `IRONFLYER_JWT_SECRET` as the orchestrator so the
  bearer-token check on `/workspaces/{id}/*` passes for tokens minted by the
  orchestrator. Rotate both together.
- The runtime Deployment uses `Recreate` strategy because workspaces are
  per-pod local filesystem state today; a rolling update would orphan them.
  Once the docker / Firecracker driver is wired up and state is externalised,
  switch to `RollingUpdate`.
- nginx-ingress annotations configure long-poll timeouts + disable buffering
  so SSE streams from `/projects/{id}/stream` and `/chat` don't stall.
- The `ingressClassName` and the example host (`ironflyer.example.com`) need
  to match your cluster — edit `ingress.yaml` before applying.
