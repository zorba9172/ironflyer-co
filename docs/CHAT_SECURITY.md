# Chat & Assistant Security Rules (constitutional)

The chat / assistant / inline-completion surfaces are **untrusted, tenant-isolated,
provider-blind**. These rules are non-negotiable. Asserted by the owner 2026-05-29.

## Rule 1 — Provider-blind: the orchestrator speaks for every vendor

The end-user must **NEVER** learn which LLM provider or model is used — not in
chat, errors, finish frames, ledgers, telemetry, completions, or any GraphQL
field or wire payload. No `anthropic`/`openai`/`gemini`/`claude`/`gpt-*`/
`deepseek`/`huggingface`/`opus`/`sonnet`/`haiku`/model ids — ever.

**Enforced at:**
- **Chat SSE** (`core/orchestrator/internal/operations/httpapi/chat_stream.go`):
  the `finish` frame carries tokens+cost only (no `provider`/`model`); errors are
  mapped to safe codes via `classifyErrorCode` + `safeChatErrorMessage` — raw
  upstream errors are never forwarded; the client's `body.Model` hint is ignored
  (the orchestrator owns routing).
- **GraphQL error presenter** (`internal/operations/gqlhardening/errors.go`):
  `reVendor` scrubs any vendor/model token to `[provider]` in **every** mode.
- **Operator-only** (return `gqlForbiddenOperator()` for non-operators): `rates`,
  `agentTelemetry`, `banditRanking`, `profitGuardDecisions`.
- **Field-blinded for users**: `costStream` (`agentCallToCostDelta`), `myBudget`
  (`ledgerEntryToGraphQL`), `ledger`/`executionLedger`/`tenantProfitToday`
  (`ledgerRowsToGraphQL` nulls LLM-leg `provider` + `scrubLedgerMetadata` drops
  `model`/`provider`/`recommended_provider` keys), `InlineCompletion`
  (no provider/model on Start/Done; generic error), `runCrew` member (no
  provider; generic error).
- **Studio (defense-in-depth)**: `ChatPanel.normalizeChatError` + `VENDOR_RE`
  collapse any vendor-naming or raw payload to a safe generic; `CostHUD`/`LogsPane`
  never render `provider`; the `LEDGER`/`RunCrew` queries don't request it; the
  agent model picker shows capability tiers (Quality/Balanced/Fast), not models.

**When adding a surface:** if a value can name a vendor/model, either gate it to
operators (`operator.IsOperator(ctx)`) or drop/redact it for users. Errors → safe
code + fixed message, never `err.Error()`.

## Rule 2 — Tenant isolation: no data bleed between users

Every read the chat/finisher feeds the model is scoped by tenant/owner; an
unscoped query returns nothing. Verified safe: chat execution owner check
(`e.TenantID != tenant → 403`; free-chat `"_"` skips execution lookup), `memory`
(all backends clamp by user/project; federation is owner-only, re-asserted at the
storage layer), `memorygraph` (errors without tenant; every read clamps
`tenant_id`), `retriever` (built from the caller's own project files),
project/workspace access (`IsAccessibleBy` + strict owner checks on mutations).

## Rule 3 — User is walled off from Ironflyer's internal systems

The chat cannot read, reuse, or leak Ironflyer's own source/secrets, and cannot
harm the platform.
- **Capability Atlas** must never index Ironflyer's own repo in a tenant-serving
  deployment (it is a shared, tenant-blind index that feeds the Coder). Self-index
  is gated behind `IRONFLYER_ATLAS_SELF_INDEX=1` (DEV ONLY); otherwise roots come
  only from `IRONFLYER_ATLAS_ROOTS`, and absent that the atlas indexes nothing.
  **TODO (follow-up):** make the Atlas tenant/project-scoped (`Capability.TenantID`
  + a scope arg on `Store.Search`) so reuse can only match the user's own code.
- **PromptGuard** (`internal/ai/providers/promptguard.go`) is wired at the top of
  the Guard (the chat path) and blocks system-prompt reveal, role hijack, and
  exfiltration of `Project.Secrets` / `*_KEY/TOKEN/SECRET/PASSWORD`.

## Production deployment gates (fail-closed) — REQUIRED before multi-tenant prod

These are NOT yet enforced at boot (dev defaults are permissive). A tenant-serving
deployment MUST set / verify:

1. **Runtime driver** = `docker` (`IRONFLYER_RUNTIME_DRIVER=docker`). The default
   `mock` driver runs exec on the host with the runtime's full env — **no isolation**.
2. **Runtime JWT** set (`IRONFLYER_JWT_SECRET`, matching the orchestrator). When
   empty the runtime disables auth and the workspace owner check is bypassed
   (any caller can address any workspace id).
3. **Policy plane** enabled (not `IRONFLYER_OPA_ALLOW_DISABLED`) — disabled means
   all decisions allow.
4. **Metrics token** set (`IRONFLYER_METRICS_TOKEN`) — provider-named Prometheus
   series are only safe behind it.
5. **Atlas** not self-indexing (`IRONFLYER_ATLAS_SELF_INDEX` unset/≠1).
6. **Project ownership**: `domain.Project.IsAccessibleBy` treats empty `OwnerID`
   as world-readable (for the public demo seed). **TODO:** add an explicit
   `Project.Public` flag and require a non-empty owner on user-created projects so
   an accidentally-blank owner fails closed instead of going public.

(See the 2026-05-29 audit for file:line detail behind each gate.)
