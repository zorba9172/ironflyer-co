# Ironflyer — Deploying to Production

End-to-end deployment guide. Covers: container build, registry push,
Kubernetes via Helm, smoke verification, post-deploy checks.

## 0. What you bring

| Need | Why | Where |
| --- | --- | --- |
| Kubernetes 1.28+ cluster | runs the workloads | EKS / GKE / AKS / DigitalOcean / Hetzner / k3s |
| Container registry | hosts the four images | GHCR (CI pushes for free) / ECR / Artifact Registry |
| Domain name | `ironflyer.example.com` is just a placeholder | Cloudflare / Route53 / anywhere |
| TLS cert | for the ingress | cert-manager + Let's Encrypt, or paste a `kubernetes.io/tls` Secret |
| Anthropic API key | the orchestrator's primary provider | https://console.anthropic.com/ |
| Stripe live keys + price IDs | for the `/budget/checkout` flow | https://dashboard.stripe.com/ |
| GitHub OAuth app | for `Continue with GitHub` | https://github.com/settings/applications/new |
| (Optional) OpenAI key | when you want OpenAI in the provider router | https://platform.openai.com/ |

## 1. Build + push images

The bundled GitHub Actions workflow at [`.github/workflows/ci.yml`](.github/workflows/ci.yml)
builds every push to `main` and pushes to `ghcr.io/<owner>/ironflyer-{orchestrator,runtime,web,code}`.
Tags: `latest` for `main`, plus `<short-sha>`.

Local build, if you'd rather:

```bash
docker build -f infra/docker/orchestrator.Dockerfile -t ghcr.io/zorba9172/ironflyer-orchestrator:v0.1 .
docker build -f infra/docker/runtime.Dockerfile      -t ghcr.io/zorba9172/ironflyer-runtime:v0.1      .
docker build -f infra/docker/web.Dockerfile          -t ghcr.io/zorba9172/ironflyer-web:v0.1          .
docker build -f infra/docker/ironflyer-code.Dockerfile -t ghcr.io/zorba9172/ironflyer-code:v0.1       .

for svc in orchestrator runtime web code; do
  docker push ghcr.io/zorba9172/ironflyer-$svc:v0.1
done
```

## 2. Helm install

```bash
# 1. Generate a 32-byte JWT secret — paste into the override below.
openssl rand -hex 32

# 2. Install. Replace placeholders.
helm upgrade --install ironflyer infra/helm/ironflyer \
  --namespace ironflyer --create-namespace \
  --set host=ironflyer.example.com \
  --set imageRegistry=ghcr.io/zorba9172 \
  --set imageTag=v0.1 \
  --set ingress.tlsSecret=ironflyer-tls \
  --set-string orchestrator.secrets.data.IRONFLYER_JWT_SECRET=<32-byte hex from above> \
  --set-string orchestrator.secrets.data.ANTHROPIC_API_KEY=sk-ant-... \
  --set-string orchestrator.secrets.data.STRIPE_SECRET_KEY=sk_live_... \
  --set-string orchestrator.secrets.data.STRIPE_WEBHOOK_SECRET=whsec_... \
  --set-string orchestrator.secrets.data.STRIPE_PRICE_PRO=price_... \
  --set-string orchestrator.secrets.data.STRIPE_PRICE_TEAM=price_... \
  --set-string orchestrator.secrets.data.STRIPE_PRICE_ENTERPRISE=price_... \
  --set-string orchestrator.secrets.data.GITHUB_CLIENT_ID=Iv1... \
  --set-string orchestrator.secrets.data.GITHUB_CLIENT_SECRET=...
```

Production tip: drop the secret block in favour of an `external-secrets`
ClusterSecretStore + ExternalSecret pointing at the real vault. Then set
`orchestrator.secrets.create=false`.

## 3. DNS + TLS

Point `A` / `AAAA` records for `ironflyer.example.com` at the ingress
controller's external IP/LoadBalancer. If you use cert-manager:

```bash
kubectl -n ironflyer apply -f - <<EOF
apiVersion: cert-manager.io/v1
kind: Certificate
metadata: { name: ironflyer-tls }
spec:
  secretName: ironflyer-tls
  dnsNames: ["ironflyer.example.com"]
  issuerRef: { name: letsencrypt-prod, kind: ClusterIssuer }
EOF
```

## 4. Stripe webhook

In Stripe Dashboard → Developers → Webhooks → Add endpoint:

- URL: `https://ironflyer.example.com/api/orchestrator/budget/webhook`
- Events: `checkout.session.completed`, `invoice.payment_succeeded`,
  `customer.subscription.deleted`, `charge.refunded`
- Copy the signing secret into `STRIPE_WEBHOOK_SECRET`.

## 5. Smoke test

```bash
ORCHESTRATOR=https://ironflyer.example.com/api/orchestrator \
RUNTIME=https://ironflyer.example.com/api/runtime \
WEB=https://ironflyer.example.com \
./scripts/smoke.sh
```

Expected: every line green except the optional `agents (auth)` row when
no `IRONFLYER_TOKEN` is exported.

## 6. Post-deploy checks

- `kubectl -n ironflyer get pods` — every pod `1/1 Running`.
- `kubectl -n ironflyer logs deploy/orchestrator` — first line should
  read `orchestrator listening` with `db=postgres`.
- Sign up with `POST /api/orchestrator/auth/signup`, then visit `/app`.
- Spin up a workspace from the Files tab, switch to the IDE tab — the
  Ironflyer-branded code-server should load in the iframe.
- Hit the Stripe Pro button on `/pricing` (logged in) — Stripe Checkout
  should open.

## 7. Rolling back

```bash
helm history -n ironflyer ironflyer
helm rollback -n ironflyer ironflyer <revision>
```

For data: Postgres lives in a StatefulSet PVC. The schema is
self-bootstrapping on boot (`BootstrapPostgres`), but DROP/CREATE
changes between releases warrant a logical dump before rollback.
