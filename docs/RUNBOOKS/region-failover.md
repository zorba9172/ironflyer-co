# Runbook — Region failover

This runbook is the operator-side flip from one prod stack to another
when the primary region goes hard-down (control plane unreachable, all
healthchecks red, on-call cannot SSH into a single pod).

The stacks are: `prod-eu` (eu-west-1), `prod-us` (us-east-1),
`prod-il` (il-central-1). The production state plane (Aurora, S3) is
per-stack; the global front door is CloudFront (AWS) or Cloudflare
(DO) routing on Route 53 / Cloudflare DNS.

## When to call a failover

- `prod-<region>` is unreachable from at least two independent vantage
  points (synthetic check + on-call laptop) for 5+ minutes.
- The orchestrator's `/livez` returns non-200 (or no answer at all) for
  every replica.
- The data plane (Aurora) is the root cause and cannot be restored
  inside the RPO window.

A single failed deploy is **not** a region failover — see
[`rollback.md`](rollback.md) instead.

## Steps

```bash
# 1. Confirm the primary is hard-down.
for r in prod-eu prod-us prod-il; do
  echo "== $r =="
  curl -sS -o /dev/null -w "%{http_code}\n" "https://api.$r.ironflyer.dev/livez" \
    --max-time 5 || echo "unreachable"
done

# 2. Mark the primary out of rotation in DNS.
#    AWS path:
aws route53 change-resource-record-sets \
  --hosted-zone-id "$(cd infra/pulumi && pulumi stack output route53ZoneID)" \
  --change-batch file://infra/dns/failover-out-<primary>.json
#    DO / Cloudflare path: flip the weighted record at the registrar.

# 3. Promote the secondary stack. Aurora is per-stack — there is no
#    cross-region replication today, so the secondary stack must already
#    carry an up-to-date snapshot restore (see DR runbook).
cd infra/pulumi
pulumi stack select <secondary>
pulumi config set ironflyer:imageTag "$(pulumi stack -s <primary> output imageTag)"
pulumi up

# 4. Smoke the secondary as the new primary.
IRONFLYER_API_URL=https://api.<secondary>.ironflyer.dev \
  bash scripts/v22_smoke.sh
```

## Post-failover

- Update the status page.
- Open an incident retro within 24h.
- Re-establish the failed primary as the new warm standby (restore
  Aurora from the snapshot used by the new primary, then `pulumi up`).
- Do **not** flip back automatically — the next failover should be a
  conscious decision.

## Known gaps (2026-05-26)

- Cross-region Aurora replication is not yet automated; the failover
  window is bounded by snapshot age.
- Wallet and ledger state must reconcile across stacks before the
  secondary accepts new paid executions — there is no global ledger
  yet. Until that lands, post-failover wallet operations must be
  paused on the failed primary so a customer cannot double-spend.
