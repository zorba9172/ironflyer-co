# GlitchTip — self-hosted error tracking for Ironflyer

GlitchTip is a self-hosted, Sentry-wire-compatible error tracker.
Ironflyer ships it inside the same Helm chart as the orchestrator so
operators can own the error-tracking data (no Sentry SaaS bill, no
third-party seeing stack traces from paid executions) without
changing a single line of the application code.

The Sentry SDKs that the services already use —
`@sentry/nextjs` in the web, `sentry-go` in the orchestrator, and the
custom envelope sender in `clients/vscode-extension/` — speak the
public Sentry protocol. GlitchTip's `/api/<project_id>/envelope/`
endpoint is wire-compatible, so the entire migration is *swap the
DSN values*. No SDK swap, no code change, no redeploy of the
application containers.

---

## Why GlitchTip over Sentry SaaS

| Concern | Sentry SaaS | GlitchTip self-hosted |
| --- | --- | --- |
| Cost at our volume | $26+/mo per user, scales by event volume | Compute in the existing DOKS cluster (~$0 incremental) |
| Data residency | US-only on free tier; EU on paid | Lives in the same Postgres / region as the rest of Ironflyer |
| Stack-trace privacy | Sent to Sentry's servers | Stays in our cluster |
| Wire protocol | Sentry envelope | Sentry envelope (drop-in) |
| Session Replay / Profiling | Yes | No (deferred — see §When to swap back) |

For Ironflyer's V22 economic model (every paid execution measured;
provider cost in the ledger), a self-hosted error store eliminates a
recurring third-party SaaS line item and keeps customer error data
inside our compliance boundary.

---

## Where it lives

- Helm chart: `infra/helm/ironflyer/templates/glitchtip.yaml`
- Values: `glitchtip:` block in
  `infra/helm/ironflyer/values.yaml` (full reference) and
  `infra/helm/ironflyer/values-prod.yaml` (prod overlay).
- DNS: `errors.<root>` A-record provisioned by
  `infra/pulumi-do/edge/cloudflare.go` (proxied through Cloudflare).
- TLS: Let's Encrypt via cert-manager (`letsencrypt-prod`
  cluster-issuer), TLS secret `glitchtip-tls`.

Public URL once deployed: <https://errors.ironflyer.ai>.

---

## Database bootstrap (one-time, before first `helm upgrade`)

GlitchTip uses its own logical database on the shared Postgres host.
The in-cluster Postgres only bootstraps the `ironflyer` database;
the managed-Postgres production stack does not bootstrap anything
beyond what Pulumi tells it to. Either way, the operator must
`CREATE DATABASE glitchtip` once before the migrate Job runs.

For managed Postgres (production):

```bash
# 1. Connect to the managed Postgres as the admin user.
psql "$POSTGRES_URL_ADMIN"

# 2. Create the database + a dedicated role.
CREATE USER glitchtip WITH PASSWORD '<strong-random-password>';
CREATE DATABASE glitchtip OWNER glitchtip;
GRANT ALL PRIVILEGES ON DATABASE glitchtip TO glitchtip;
\q

# 3. Wire the DSN into the prod values.
export GLITCHTIP_DATABASE_URL="postgres://glitchtip:<pw>@<managed-host>:25060/glitchtip?sslmode=require"
```

For the in-cluster Postgres (dev / staging) the chart auto-derives
the DSN from `.Values.postgres.*`, but the `glitchtip` database
itself still has to exist. Quick path inside the Postgres pod:

```bash
kubectl -n ironflyer exec -it postgres-0 -- \
  psql -U ironflyer -d ironflyer -c "CREATE DATABASE glitchtip OWNER ironflyer;"
```

---

## First deploy — admin signup

1. **Helm upgrade with registration open.** `values-prod.yaml` ships
   with `glitchtip.enableUserRegistration: true` so the very first
   `helm upgrade` brings GlitchTip up in a state where the operator
   can sign up:

   ```bash
   helm upgrade --install ironflyer infra/helm/ironflyer \
     -n ironflyer --create-namespace \
     -f infra/helm/ironflyer/values.yaml \
     -f infra/helm/ironflyer/values-prod.yaml \
     --set-string glitchtip.databaseUrlOverride="$GLITCHTIP_DATABASE_URL"
   ```

2. **Wait for cert-manager.** The first request to
   `https://errors.ironflyer.ai` may 404 for ~60s while
   cert-manager solves the HTTP01 challenge. Re-check with
   `kubectl -n ironflyer get certificate glitchtip-tls` —
   `READY=True` is the green light.

3. **Sign up the admin user.** Open
   <https://errors.ironflyer.ai/accounts/signup/>, fill the form.
   The first account on a fresh install becomes a superuser.

4. **Create the organization.** Top-right org switcher →
   "Create Organization". Name it `ironflyer`.

5. **Create three projects, one per service.** Settings → Projects:
   - `orchestrator` — platform: Go
   - `web` — platform: JavaScript (Next.js)
   - `vscode-extension` — platform: Node

   Each project page shows a DSN that looks like
   `https://<public-key>@errors.ironflyer.ai/<project-id>`. Copy
   each one.

6. **Wire the DSNs into the Pulumi config layer.** Edit
   `.env.production.local` (or the equivalent secrets file the
   `load-secrets-to-pulumi.sh` script reads) and set:

   ```bash
   SENTRY_DSN_ORCHESTRATOR=https://<key>@errors.ironflyer.ai/<id>
   SENTRY_DSN_WEB=https://<key>@errors.ironflyer.ai/<id>
   NEXT_PUBLIC_SENTRY_DSN=https://<key>@errors.ironflyer.ai/<id>
   SENTRY_DSN_VSCODE_EXTENSION=https://<key>@errors.ironflyer.ai/<id>
   ```

   Then re-run the loader so Pulumi picks them up on the next
   `pulumi up`:

   ```bash
   bash scripts/load-secrets-to-pulumi.sh prod-ams3
   ```

7. **Harden — close registration.** Edit
   `infra/helm/ironflyer/values-prod.yaml` and flip
   `glitchtip.enableUserRegistration: false`. Re-run
   `helm upgrade ...`. GlitchTip will refuse new signups; the
   admin can still invite team members from the dashboard.

---

## Validation

After the next service redeploy, trigger an intentional error from
each service and confirm it lands in the matching GlitchTip project:

- Orchestrator: hit a known-broken admin endpoint (or run
  `kubectl -n ironflyer exec deploy/orchestrator -- /app/cli sentry-test`).
- Web: visit `https://ironflyer.ai/dev/throw` (the dev-only
  intentional-throw page).
- VSCode extension: install the dev build, run
  `Ironflyer: Throw test error` from the command palette.

Each error should appear in its project within ~5s.

---

## Backups and retention

- **Backups** — GlitchTip's data lives in the `glitchtip` Postgres
  database, which sits on the same managed Postgres cluster as
  `ironflyer`. The existing DO Postgres daily snapshot + wal-g WAL
  archiving (see `docs/DR_RUNBOOK.md`) covers it automatically.
  No separate backup configuration required.
- **Retention** — GlitchTip auto-prunes events older than
  `GLITCHTIP_MAX_EVENT_LIFE_DAYS` (default 90). Per-project event
  quotas can be set from the project Settings → Subscription page.
  For our launch volume the default is fine; revisit when the
  Postgres `events_event` table crosses ~10M rows.

---

## When to swap back to Sentry SaaS

GlitchTip covers exception tracking, performance transactions, and
release health. It does NOT (as of v4.x) ship:

- **Session Replay** — DOM playback of the user session preceding
  the error.
- **Profiling** — CPU/memory flame graphs sampled in production.
- **AI-assisted issue grouping / similarity dedup** — Sentry has a
  proprietary model here; GlitchTip uses pure stack-trace fingerprinting.

If/when we need any of those, swap one or more of the
`SENTRY_DSN_*` env vars back to the matching Sentry SaaS DSN.
Because the SDKs are unchanged, the switch is purely an
environment-variable rotation through Pulumi + redeploy. We can
even run them side-by-side temporarily by configuring two SDK
clients (one per DSN), but for cost reasons we don't.

---

## Operational notes

- **Image pin.** Production runs `glitchtip/glitchtip:v4.1`
  (set in `values.yaml`). Bumping requires reading the upstream
  changelog at <https://gitlab.com/glitchtip/glitchtip-backend>
  for migrations + breaking changes.
- **SECRET_KEY rotation.** The chart derives `SECRET_KEY`
  deterministically from the release name. To force a rotation
  (invalidates all sessions + signed URLs) set
  `--set-string glitchtip.secretKeyOverride="$(openssl rand -hex 32)"`.
- **Redis isolation.** GlitchTip's Celery broker uses a dedicated
  `glitchtip-redis` Deployment so a `FLUSHALL` here cannot wipe
  application Redis state. The data is in-memory only (no
  persistence) — losing it just re-queues the next Celery beat tick.
- **Resource ceiling.** HPA capped at 3 web replicas; this is an
  internal ops tool, not a customer-facing service. Raise via
  `glitchtip.autoscaling.maxReplicas` if event volume grows enough
  to push CPU past the 70% target.
