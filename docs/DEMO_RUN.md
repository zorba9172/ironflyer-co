# End-to-end demo run captured 2026-05-26

This document is the verbatim transcript of one realistic paying-user
flow through the live stack on 2026-05-26 (orchestrator @ :8080,
runtime @ :8090, postgres @ :5432, mock provider — `ANTHROPIC_API_KEY`
unset). The script `scripts/demo/run-stridon-click.sh` reproduces every
request below. Only GraphQL was used; no REST endpoints were touched.

> **Prompt under test (verbatim):**
> *Build a marketing landing page for a fictional ergonomic mechanical
> keyboard brand called 'Stridon Click', highlighting the ortholinear
> layout, recycled-aluminum chassis, and Bluetooth+USB-C hybrid
> wireless.*

## Identifiers produced by this run

| key             | value                                            |
|-----------------|--------------------------------------------------|
| user email      | `demo+1779748689@ironflyer.local`                |
| user / tenant   | `5ee566e1-7fe2-48e6-bc2c-534a274a20c5`           |
| project ID      | `222bb600-ae32-4801-8672-0b1076f4e948`           |
| execution ID    | `4315bd65-434f-41f8-ad71-67398d56f83f`           |
| blueprint       | `static-landing` (picked by ideaparser keyword)  |
| budget held     | $2.00 (suggestedBudgetUSD)                       |
| wallet seed     | $50 (`IRONFLYER_DEV_WALLET_SEED_USD`)            |
| final status    | `stopped` (`failure_reason="demo timeout"`)      |

---

## Step 1 — `mutation signUp`

A fresh user is created so the run is hermetic from any prior state.

Request:

```graphql
mutation SignUp($email: String!, $password: String!) {
  signUp(input: {email: $email, password: $password, name: "Stridon Demo"}) {
    token
    user { id email plan }
  }
}
```

Response (token truncated for brevity, full value lives in
`scripts/demo/run-stridon-click.sh`):

```json
{
  "data": {
    "signUp": {
      "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
      "user": {
        "id": "5ee566e1-7fe2-48e6-bc2c-534a274a20c5",
        "email": "demo+1779748689@ironflyer.local",
        "plan": "free"
      }
    }
  }
}
```

The auth path returns a JWT (HS256) plus a `free` plan, and the wallet
row is auto-seeded by `IRONFLYER_DEV_WALLET_SEED_USD=50`.

## Step 2 — `query wallet` (initial)

```json
{
  "data": {
    "wallet": {
      "tenantID": "5ee566e1-7fe2-48e6-bc2c-534a274a20c5",
      "balanceUSD": 50,
      "holdUSD": 0,
      "availableUSD": 50,
      "lifetimeTopUpUSD": 50,
      "updatedAt": "2026-05-26T01:38:09.871671+03:00"
    }
  }
}
```

Wallet starts at $50 / $0 hold / $50 available — the dev seed already
credited the new tenant, so no Stripe top-up is needed.

## Step 3 — `mutation describeIdea`

The V22 vibe-builder entrypoint (per `studio.resolver.go:DescribeIdea`).
A single call validates auth, parses the idea, picks the blueprint,
creates the project, places the wallet hold (law 1) and admits +
starts the execution.

```graphql
mutation Describe($text: String!) {
  describeIdea(input: {text: $text}) {
    project { id name }
    execution {
      id status budgetUSD reservedUSD spentUSD
      blueprintID workspaceID projectID metadata
    }
    idea {
      title summary blueprintID blueprintReason
      suggestedBudgetUSD stopLossUSD confidence tags
    }
    costEstimate { __typename }
  }
}
```

Response:

```json
{
  "data": {
    "describeIdea": {
      "project": {
        "id": "222bb600-ae32-4801-8672-0b1076f4e948",
        "name": "Build a marketing landing page for"
      },
      "execution": {
        "id": "4315bd65-434f-41f8-ad71-67398d56f83f",
        "status": "running",
        "budgetUSD": 2,
        "reservedUSD": 0,
        "spentUSD": 0,
        "blueprintID": "static-landing",
        "workspaceID": null,
        "projectID": "222bb600-ae32-4801-8672-0b1076f4e948"
      },
      "idea": {
        "title": "Build a marketing landing page for",
        "summary": "Build a marketing landing page for a fictional ergonomic mechanical keyboard brand called 'Stridon Click', highlighting the ortholinear layout, recycled-aluminum chassis, and Bluetooth+USB-C hybrid wireless",
        "blueprintID": "static-landing",
        "blueprintReason": "keyword 'landing' → static-landing",
        "suggestedBudgetUSD": 2,
        "stopLossUSD": 3,
        "confidence": 0.65,
        "tags": ["static"]
      }
    }
  }
}
```

The ideaparser (LLM unavailable in dev, rules fallback) matched
`landing` and routed to the `static-landing` blueprint with
confidence=0.65, suggesting a $2 budget and a $3 stop-loss.

## Step 4 — `query wallet` (post-describe — the hold is real)

```json
{
  "data": {
    "wallet": {
      "balanceUSD": 50,
      "holdUSD": 2,
      "availableUSD": 48
    }
  }
}
```

Law 1 verified: `holdUSD` jumped from $0 → $2 the instant the
execution was admitted, and `availableUSD` dropped by the same amount.
This is the customer-visible proof that no model call can fire without
budget in escrow.

## Step 5 — `subscription executionFeed`

Subscribed via `graphql-transport-ws` with the JWT in the
`connection_init.payload.authorization` field. The transport ack was
received cleanly:

```json
{"meta":"ack","msg":{"type":"connection_ack"}}
```

…and the stream was held open for 90 s. Zero `next` payloads arrived.
This is consistent with the postgres `execution_events` view (which is
what the resolver tails):

```text
 event_type |          created_at
------------+-------------------------------
 created    | 2026-05-25 22:38:26.743007+00
 admitted   | 2026-05-25 22:38:26.757283+00
 started    | 2026-05-25 22:38:26.759412+00
 stopped    | 2026-05-25 22:43:19.562684+00
```

i.e. only the four FSM lifecycle events were produced. The embedded
finisher executor never emitted a workspace allocation, a patch, a
gate result, or a cost-added event before the demo timeout fired —
matching `workspaceID=null` and `spentUSD=0` on the execution row. The
mock-provider engine logged repeated `graphql: panic in resolver`
errors in `/tmp/orch.log` during the same window, suggesting the
engine loop crashed silently and was restarted with no work checkpointed.

## Step 6 — `mutation stopExecution`

The capture window closed; we stop the run rather than fake progress:

```graphql
mutation { stopExecution(id: "4315bd65-...", reason: "demo timeout") { ... } }
```

Response:

```json
{
  "data": {
    "stopExecution": {
      "id": "4315bd65-434f-41f8-ad71-67398d56f83f",
      "status": "stopped",
      "spentUSD": 0,
      "reservedUSD": 0,
      "refundedUSD": 0,
      "endedAt": "2026-05-26T01:43:19.549176+03:00",
      "failureReason": "demo timeout"
    }
  }
}
```

## Step 7 — `query executionSupportBundle`

The 6-artifact wow-loop bundle (Agent 34 / `internal/wowloop`):

```json
{
  "data": {
    "executionSupportBundle": {
      "executionID": "4315bd65-434f-41f8-ad71-67398d56f83f",
      "tenantID": "5ee566e1-7fe2-48e6-bc2c-534a274a20c5",
      "status": "stopped",
      "previewURL": null,
      "productionURL": null,
      "changedFiles": [],
      "patchCount": 0,
      "gateReport": { "completionScore": 0, "stages": [] },
      "securityReport": {
        "passRate": 1,
        "blockedDeploy": false,
        "findings": []
      },
      "costReport": {
        "revenueUSD": 0,
        "providerCostUSD": 0,
        "sandboxCostUSD": 0,
        "storageCostUSD": 0,
        "deploymentCostUSD": 0,
        "grossMarginPct": 0
      },
      "nextBestAction": {
        "kind": "review_patch",
        "title": "Review the patches that landed",
        "reason": "Walk through the applied patches to confirm intent before kicking off the next iteration.",
        "cta": "/app/executions/4315bd65-434f-41f8-ad71-67398d56f83f#patches"
      },
      "generatedAt": "2026-05-26T01:43:39.333855+03:00"
    }
  }
}
```

The bundle resolver returns a fully-populated, non-null shape even on
a no-progress run — the customer never sees a stale or missing
artifact, only zeroed values.

## Step 8 — `query execution` (final row)

```json
{
  "data": {
    "execution": {
      "id": "4315bd65-434f-41f8-ad71-67398d56f83f",
      "status": "stopped",
      "budgetUSD": 2,
      "reservedUSD": 0,
      "spentUSD": 0,
      "refundedUSD": 0,
      "revenueUSD": 0,
      "providerCostUSD": 0,
      "sandboxCostUSD": 0,
      "storageCostUSD": 0,
      "deploymentCostUSD": 0,
      "completionScore": 0,
      "grossMarginPct": null,
      "workspaceID": null,
      "projectID": "222bb600-ae32-4801-8672-0b1076f4e948",
      "failureReason": "demo timeout",
      "createdAt":  "2026-05-26T01:38:26.736478+03:00",
      "admittedAt": "2026-05-26T01:38:26.753886+03:00",
      "startedAt":  "2026-05-26T01:38:26.756957+03:00",
      "endedAt":    "2026-05-26T01:43:19.549176+03:00"
    }
  }
}
```

Postgres confirms the row independently:

```text
                  id                  | status  | budget_usd | spent_usd | refunded_usd | failure_reason
--------------------------------------+---------+------------+-----------+--------------+----------------
 4315bd65-434f-41f8-ad71-67398d56f83f | stopped |   2.000000 |  0.000000 |     0.000000 | demo timeout
```

## Step 9 — `query wallet` (final)

```json
{
  "data": {
    "wallet": {
      "balanceUSD": 50,
      "holdUSD": 2,
      "availableUSD": 48,
      "lifetimeTopUpUSD": 50,
      "lifetimeSpendUSD": 0
    }
  }
}
```

Observation: `stopExecution` did **not** release the $2 hold (still
$2, available still $48, lifetime spend still $0). The settler / refund
path on a no-progress stop appears not to have run — that is a real
gap captured by this transcript, not a demo artefact.

## Step 10 — `query profitGuardDecisions`

```json
{ "data": { "profitGuardDecisions": [] } }
```

No ProfitGuard decisions were recorded for this execution. Expected,
since the engine never reached an enforcement point (no
`pre_step` / `post_step` / `deploy_gate` calls were made).

---

## The 6 wow-loop artifacts — actual content

| # | Artifact          | Source field                              | Captured value                                                                                                                                  |
|---|-------------------|-------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------|
| 1 | Preview URL       | `executionSupportBundle.previewURL`       | `null` — not produced (engine never allocated a workspace; `workspaceID=null`).                                                                  |
| 2 | Changed files     | `executionSupportBundle.changedFiles`     | `[]` (and `patchCount=0`). Not produced — no patches landed.                                                                                     |
| 3 | Gate report       | `executionSupportBundle.gateReport`       | `{ completionScore: 0, stages: [] }` — bundle shape returned, but no gate stages ran.                                                            |
| 4 | Security report   | `executionSupportBundle.securityReport`   | `{ passRate: 1.0, blockedDeploy: false, findings: [] }` — vacuously passing because nothing was scanned.                                         |
| 5 | Cost report       | `executionSupportBundle.costReport`       | `{ revenueUSD: 0, providerCostUSD: 0, sandboxCostUSD: 0, storageCostUSD: 0, deploymentCostUSD: 0, grossMarginPct: 0 }` — zeroes, no spend.       |
| 6 | Next best action  | `executionSupportBundle.nextBestAction`   | `{ kind: "review_patch", title: "Review the patches that landed", cta: "/app/executions/4315bd65-…#patches" }` — generic default returned.       |

## Honest summary

The economic contract works end-to-end: signup → wallet seed →
`describeIdea` → wallet hold ($2 placed, visible in wallet,
postgres row exists) → `stopExecution` → `executionSupportBundle`
returning a fully-shaped JSON response. The finisher engine itself
did not reach `succeeded` on the mock provider — it parked in
`running` for the entire 90 s capture window without emitting a
single workspace, patch, gate, or cost event. The orchestrator log
shows repeated `ERR graphql: panic in resolver panic={}` lines
during that window, which is the most likely cause.

Two real gaps fall out of this honest run, both worth fixing before
the next demo:

1. **Engine doesn't move on the mock provider** — `static-landing`
   admitted + started but produced zero progress events. Either the
   blueprint's first step needs a non-LLM no-op for the dev fallback,
   or the engine should fail-fast and mark `failed` instead of
   silently parking in `running`.
2. **`stopExecution` does not release the wallet hold** — after the
   stop, `holdUSD` is still $2 and `lifetimeSpendUSD` is still $0,
   meaning the hold leaks until manual refund. This violates the
   "hold released on terminal state" invariant the wallet docs imply.

`scripts/demo/run-stridon-click.sh` reproduces the whole transcript
above against any fresh tenant in under two minutes.
