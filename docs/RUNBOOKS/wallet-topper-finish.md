# Wallet Topper — finish-line runbook (Stripe + Paddle)

Status as of 2026-05-27 commit `a1bdcbf6`:
- **Code is fully wired.** Multi-provider `wallet.Topper` interface,
  `StripeTopper`, `PaddleTopper`, `TopperRegistry`, `Reconciler` (5-min
  cron), `Idempotency-Key` on Stripe POSTs, dual webhook routes, and
  `walletAvailableProviders` + `walletCreateTopUp(amountUSD, provider)`
  GraphQL are all in `main`.
- **⚠ The production orchestrator binary is older than this code.**
  Probing `walletAvailableProviders` against `api.ironflyer.ai` today
  returns `Cannot query field "walletAvailableProviders" on type "Query"`
  — that's the stale binary, not a schema bug. Step A.3 (rolling the
  orchestrator pair on a fresh image) is therefore mandatory **before**
  the env edits alone can take effect.
- **Compose is wired.** `infra/compose/docker-compose.prod.yml` forwards
  every Stripe + Paddle + primary-provider env var to the orchestrator
  pair.
- **DB migration is in place** — `00046_wallet_topups_provider.sql`
  adds the `provider` column with the right backfill.
- **UI is shipped.** `/wallet/topup` reads `walletAvailableProviders`,
  renders primary CTA + "Card declined or in a blocked region?"
  alternatives, and handles both `?session_id=cs_*` (Stripe) and
  `?_ptxn=txn_*` (Paddle) return URLs through the same poll-and-confirm
  view.

The only outstanding work is **operator config + redeploy** — no more
code to write. This document is the runbook to get there.

---

## Path A — start with Paddle alone (Stripe verification still pending)

You can ship paid execution **today** using only Paddle. The
`PADDLE_API_KEY` in `.env.production.local` is already live; the
remaining step is the webhook secret.

### A.1 — Register the wallet-topup webhook in Paddle

1. Sign in at <https://vendors.paddle.com>.
2. **Developer Tools → Notifications → New destination**.
3. Settings:
   - **Notification URL:** `https://api.ironflyer.ai/budget/paddle/webhook`
   - **Description:** `Ironflyer wallet topper (production)`
   - **Active:** ✅ on
   - **Events to subscribe to** — minimum set:
     - `transaction.completed`
     - `transaction.paid`
     - `transaction.payment_failed`
     - `transaction.canceled`
     - `subscription.activated` (only if you also want subscription billing later)
4. Click **Save**. Paddle reveals the **Secret key** (starts with `pdl_ntfset_…`). Copy it.

### A.2 — Add the secret + Paddle config to `infra/compose/.env.prod` on the AX102 host

Open an SSH session and append:

```bash
ssh ironflyer@<AX102-IP>
cd /path/to/ironflyer/infra/compose
# Append the wallet-topper block; `>>` is safe because docker-compose
# reads the last value if a key repeats.
cat >> .env.prod <<'EOF'

# --- Wallet topper (Paddle only — Stripe pending verification) ---------------
PADDLE_API_KEY=<copy from .env.production.local on your laptop>
PADDLE_WEBHOOK_SECRET=<paste from step A.1>
PADDLE_ENV=live
PADDLE_WALLET_SUCCESS_URL=https://app.ironflyer.ai/wallet/topup
PADDLE_WALLET_CANCEL_URL=https://app.ironflyer.ai/wallet/topup?cancelled=1
IRONFLYER_WALLET_PRIMARY_PROVIDER=paddle
EOF
chmod 600 .env.prod
```

> The real `PADDLE_API_KEY` lives in `.env.production.local` on your
> laptop. **Don't paste it into this runbook** — GitHub's secret
> scanner blocks it on push. Either render the block with
> `bash scripts/apply-wallet-secrets-to-prod.sh -o /tmp/topper.env`
> (which reads the key from `.env.production.local` for you), then
> `scp /tmp/topper.env` to AX102; or copy the key manually inside an
> SSH session.

### A.3 — Roll the orchestrator pair

```bash
docker compose -f docker-compose.prod.yml --env-file .env.prod \
  pull orchestrator-1 orchestrator-2
docker compose -f docker-compose.prod.yml --env-file .env.prod \
  up -d orchestrator-1 orchestrator-2
# Tail until both are healthy
docker compose -f docker-compose.prod.yml --env-file .env.prod \
  ps orchestrator-1 orchestrator-2
```

### A.4 — Verify from your laptop

```bash
cd /Users/moshemoran/workspace/ironflyer-copilot/ironflyer
bash scripts/check-wallet-providers.sh https://api.ironflyer.ai
# Expect:
#   ✓ walletAvailableProviders returned 1 provider: paddle (primary)
#   ✓ walletCreateTopUp returned a Paddle checkout URL
```

Done — Paddle is live.

---

## Path B — add Stripe once verification completes

When Stripe finishes your identity check and you can issue keys:

### B.1 — Create a restricted live key in Stripe

1. <https://dashboard.stripe.com/apikeys> → **+ Create restricted key**.
2. Permissions (exactly these scopes; everything else stays `None`):
   - `Charges`: **Write**
   - `Customers`: **Write**
   - `Checkout sessions`: **Write**
   - `Subscriptions`: **Write**
   - `Prices`: **Read**
   - `Products`: **Read**
   - `Refunds`: **Write**
   - `Webhook endpoints`: **Write** (only needed if you'll register
     the webhook via API; can skip if you'll create the webhook in
     the dashboard).
3. Name it `Ironflyer prod wallet topper`. Click **Create key**. Copy
   the `rk_live_…` value.

### B.2 — Register the webhook

1. <https://dashboard.stripe.com/webhooks> → **+ Add endpoint**.
2. Endpoint URL: `https://api.ironflyer.ai/budget/webhook`
3. Description: `Ironflyer wallet topper`
4. Events to listen for: **`checkout.session.completed`** (that's the
   only event `wallet.StripeTopper.HandleWebhook` cares about; adding
   more is fine but unnecessary).
5. Click **Add endpoint**. Reveal the **Signing secret** (`whsec_…`).
   Copy it.

### B.3 — Patch `.env.prod` on the AX102

```bash
ssh ironflyer@<AX102-IP>
cd /path/to/ironflyer/infra/compose
cat >> .env.prod <<'EOF'

# --- Stripe leg (added after verification cleared) ----------------------------
STRIPE_SECRET_KEY=rk_live_<from-step-B.1>
STRIPE_WEBHOOK_SECRET=whsec_<from-step-B.2>
STRIPE_SUCCESS_URL=https://app.ironflyer.ai/wallet/topup?session_id={CHECKOUT_SESSION_ID}
STRIPE_CANCEL_URL=https://app.ironflyer.ai/wallet/topup?cancelled=1
EOF
chmod 600 .env.prod
```

> **Decide which is primary.** Leave `IRONFLYER_WALLET_PRIMARY_PROVIDER=paddle`
> if Israeli users dominate (Paddle handles IL VAT cleanly); flip to
> `stripe` if you want Stripe as default. Either way, both providers
> are reachable from `/wallet/topup` via the user-driven failover link.

### B.4 — Roll + verify

```bash
docker compose -f docker-compose.prod.yml --env-file .env.prod up -d \
  orchestrator-1 orchestrator-2

# From your laptop
bash scripts/check-wallet-providers.sh https://api.ironflyer.ai
# Expect:
#   ✓ walletAvailableProviders returned 2 providers: paddle, stripe
#   ✓ walletCreateTopUp(provider:"paddle") -> Paddle URL
#   ✓ walletCreateTopUp(provider:"stripe") -> Stripe URL
```

---

## Path C — VSCode Marketplace publish (independent of payments)

Already prepared end-to-end. Only thing pending is the publisher PAT.

### C.1 — Verify the publisher

1. Sign in to <https://marketplace.visualstudio.com/manage> with the
   Microsoft / Azure DevOps account that should own the `ironflyer`
   publisher.
2. If the publisher doesn't exist → **+ Create publisher**:
   - Publisher ID: `ironflyer` (must exactly match `publisher` in
     [package.json](../../clients/vscode-extension/package.json#L5))
   - Display name: `Ironflyer`
   - Optionally upload the same icon as `clients/vscode-extension/media/icon.png`.

### C.2 — Issue a Marketplace PAT

1. <https://dev.azure.com> → top-right user icon → **Personal access tokens**.
2. **+ New token**:
   - Name: `Ironflyer VSCode Marketplace publish`
   - Organization: **All accessible organizations** (required for
     `vsce publish`; the Marketplace lives in the shared MS tenant).
   - Expiration: 90 days (recommended; rotate via this same flow).
   - Scopes: **Custom defined** → expand **Marketplace** → tick
     **Manage**. Untick everything else.
3. **Create** → copy the token. Stripe-style: it's shown once.

### C.3 — Park the PAT in GitHub secrets

```bash
gh secret set VSCE_PAT --body "<token-from-step-C.2>" \
  --repo zorba9172/ironflyer-co
# Optional: also set OVSX_PAT (open-vsx.org → Settings → Access Tokens)
gh secret set OVSX_PAT --body "<ovsx-token>" --repo zorba9172/ironflyer-co
```

### C.4 — Cut the release

```bash
cd /Users/moshemoran/workspace/ironflyer-copilot/ironflyer
# The version in clients/vscode-extension/package.json must match.
git tag -a vscode-v0.3.1 -m "Marketplace listing metadata fixes"
git push origin vscode-v0.3.1
```

The `release-vscode.yml` workflow will:
1. Install, typecheck, lint, build, package the .vsix.
2. Upload the `.vsix` as a GitHub Actions artifact.
3. Publish to VS Marketplace (only when `VSCE_PAT` is set).
4. Publish to Open VSX (only when `OVSX_PAT` is set).
5. Create a GitHub Release with the `.vsix` attached.

Watch <https://github.com/zorba9172/ironflyer-co/actions> until the run
goes green. The listing lands at:
`https://marketplace.visualstudio.com/items?itemName=ironflyer.ironflyer`.

---

## Reference — the exact values you'll need to keep handy

| Value | Where it lives now | What it becomes in prod |
|---|---|---|
| `PADDLE_API_KEY` | `.env.production.local:35` | `infra/compose/.env.prod` on AX102 |
| `PADDLE_WEBHOOK_SECRET` | empty (Paddle dashboard step A.1) | `infra/compose/.env.prod` on AX102 |
| `STRIPE_SECRET_KEY` | empty (Stripe dashboard step B.1) | `infra/compose/.env.prod` on AX102 |
| `STRIPE_WEBHOOK_SECRET` | empty (Stripe dashboard step B.2) | `infra/compose/.env.prod` on AX102 |
| `IRONFLYER_WALLET_PRIMARY_PROVIDER` | new (decision) | `infra/compose/.env.prod` on AX102 |
| `VSCE_PAT` | Azure DevOps PAT (step C.2) | `gh secret set VSCE_PAT` |
| `OVSX_PAT` | open-vsx.org PAT (optional) | `gh secret set OVSX_PAT` |

Everything else (`ANTHROPIC_API_KEY`, `GITHUB_APP_*`, `DIGITALOCEAN_TOKEN`,
`CLOUDFLARE_*`, `LEMONSQUEEZY_API_KEY`, etc.) is already in
`.env.production.local` and gets pushed to `.env.prod` via
[scripts/load-secrets-to-pulumi.sh](../../scripts/load-secrets-to-pulumi.sh)
or whichever provisioning path you use. **The runbook above only adds
the wallet-topper-specific values that need a human step in a vendor
dashboard.**
