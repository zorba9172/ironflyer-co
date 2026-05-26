# Paid Customer Launch Runbook

This runbook is the operating checklist for serving paying customers with
profitable production behavior from the first day.

## Non-Negotiables

- Billing and wallet holds must be live before any paid execution starts.
- ProfitGuard must be wired on model calls, artifact storage, deploy, and long
  verification points.
- AppSec scans only the customer workspace, never Ironflyer source.
- Domain purchase stays disabled until registrar credentials, billing caps, and
  operator policy are confirmed.
- Audit export must be available for policy decisions, deploy actions, billing
  actions, and secret releases.

## Domain Purchase Enablement

Keep purchase disabled unless all items below are true:

- Registrar account has a production billing profile.
- Tenant/project secret `CLOUDFLARE_API_TOKEN` exists in the secrets broker.
- `CLOUDFLARE_ACCOUNT_ID` is set for the orchestrator.
- `IRONFLYER_DOMAIN_PURCHASE_ENABLED=true`.
- `IRONFLYER_DOMAIN_MAX_PURCHASE_USD` is set to a margin-safe value.
- `IRONFLYER_DOMAIN_PRICE_TOLERANCE_PCT` is set deliberately.
- Customer-facing terms explain domain purchase, renewal, and refund policy.

Recommended first production posture:

```text
IRONFLYER_DOMAIN_REGISTRAR=cloudflare
IRONFLYER_DOMAIN_PURCHASE_ENABLED=true
IRONFLYER_DOMAIN_MAX_PURCHASE_USD=40
IRONFLYER_DOMAIN_PRICE_TOLERANCE_PCT=5
IRONFLYER_DOMAIN_REQUIRE_CONTACT=false
```

Use `IRONFLYER_DOMAIN_REQUIRE_CONTACT=true` only after the UI collects and
validates registrant contact data.

## First-Customer Verification

Run these checks before opening paid access:

```bash
go test ./...
npm run typecheck
```

Then verify manually:

- A paid execution cannot start without a wallet hold.
- A stopped execution releases or settles funds correctly.
- A deploy cannot promote when security gates block it.
- A live deploy receives a managed subdomain.
- A custom domain returns DNS records without exposing provider credentials.
- Domain purchase fails cleanly when purchase is disabled.
- Domain purchase fails when the server-side quote exceeds the cap.
- Audit rows exist for policy decisions and secret releases.

## Launch Metrics

Watch these every hour on launch day:

- Gross margin per execution.
- Refund rate.
- Failed deploy rate.
- Domain purchase failures by reason.
- AppSec blocking rate.
- Average time to live preview.
- Wallet hold failures.
- Provider spend by model/deploy target.

## Stop Conditions

Pause paid acquisition when any of these happen:

- Profit margin trends below the operator threshold.
- Domain purchases produce registrar errors or unexpected prices.
- Audit writes fail.
- Secret release fails open.
- AppSec scans include non-customer paths.
- Deploys succeed without wallet/profitguard records.

