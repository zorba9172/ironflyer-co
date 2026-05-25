# Ironflyer Policy, Security, and Trust Architecture

> V22 trust contract: Ironflyer is not a vibe builder. It is a paid AI
> execution engine with a single policy plane, enforced gates, tenant isolation,
> auditable decisions, and production-data boundaries that AI cannot bypass.

This document defines the policy/security architecture that sits across the
GraphQL API, orchestrator, provider router, runtime workspaces, patch lifecycle,
deploy path, wallet/ledger, and audit chain. It is an implementation contract
for the V22 architecture in `ARCHITECTURE.md` and `docs/V22_PLAN.md`.

## Non-Negotiable Trust Laws

1. **Deny by default.** Every consequential action needs an allow decision from
   the policy plane.
2. **One policy plane.** Services do not invent local authorization rules. They
   call the Policy Decision Point and log the decision ID.
3. **AI is never a principal of record.** AI acts as a delegated tool under a
   user, tenant, execution, workspace, and wallet context.
4. **AI never receives ambient power.** No production credentials, deploy tokens,
   customer data exports, or host filesystem access are available inside model
   context or workspace processes.
5. **Patch lifecycle is mandatory.** AI proposes; policy, gates, and approvals
   decide; runtime applies only approved patches.
6. **Production data is isolated from execution sandboxes.** Synthetic,
   anonymized, or tenant-approved fixtures are the default test input.
7. **Every high-risk decision is audit chained.** Authz, command execution,
   secret release, deploy approvals, policy denies, and abuse actions must be
   replayable from the hash chain.
8. **Economics and security are peers.** ProfitGuard decides whether the action
   is worth paying for; Policy decides whether the action is allowed.

## Policy Plane Topology

```
GraphQL / WS
  -> AuthN
  -> request hardening
  -> PEP: graph operation
  -> resolver
       -> PEP: domain action
       -> ProfitGuard for paid/expensive steps
       -> provider router / runtime / secret broker / deploy adapter
            -> PEP: concrete side effect
            -> audit.Record(policy_decision_id, outcome)
```

| Component | Role |
| --- | --- |
| **PDP** | Central Policy Decision Point. Evaluates action, principal, resource, tenant, environment, abuse, and economic context. |
| **PEP** | Policy Enforcement Point embedded in GraphQL, provider router, runtime, patch engine, secret broker, deploy adapter, and admin tools. |
| **Policy bundle store** | Versioned policy bundles, signed at release time, loaded read-only by all orchestrator pods. |
| **Policy decision log** | Append-only table plus audit-chain event for every deny and every high-risk allow. |
| **Risk engine** | Computes abuse score and action risk before PDP evaluation. |
| **Secret broker** | The only component allowed to unwrap secrets; releases scoped capabilities, never raw secrets to AI. |
| **Audit store** | Hash-chained record of consequential events, decision IDs, content hashes, and operator approvals. |

The PDP response shape is intentionally small and stable:

```json
{
  "decision_id": "pdec_...",
  "effect": "allow",
  "risk": "medium",
  "reason": "tenant_admin_can_deploy_after_gate_pass",
  "obligations": [
    "audit.high_risk_allow",
    "require_deploy_approval_id",
    "redact_model_context"
  ],
  "ttl_seconds": 60
}
```

No side effect may continue until all obligations are satisfied.

## OPA / Cedar Decision

Ironflyer uses **OPA as the runtime PDP** for V22 because it is mature for
service-side authorization, Kubernetes/infra admission, Rego bundles, decision
logs, and CI policy tests.

Cedar is reserved for the **application authorization model** where its shape is
strongest: principal/action/resource decisions such as "user can read project",
"tenant admin can approve deploy", and "service account can rotate secret".
Those Cedar policies compile into or are invoked by the OPA PDP so the product
still has one decision endpoint and one decision log.

| Layer | Language | Examples |
| --- | --- | --- |
| Service, runtime, GraphQL, deploy, abuse, environment | OPA/Rego | command allowlists, deploy gating, production data isolation, risk thresholds |
| App RBAC/ABAC | Cedar-compatible model | user, tenant, project, workspace, execution, role, ownership, delegated approval |
| Economic gating | ProfitGuard domain logic | continue, degrade, switch_provider, pause, stop, kill_branch |

OPA is the integration point; Cedar is the app-authz policy model. The system
never exposes two competing answers to callers.

## Policy Input Contract

Every PEP sends the same envelope:

```json
{
  "principal": {
    "kind": "user",
    "user_id": "usr_...",
    "tenant_id": "ten_...",
    "roles": ["tenant_admin"],
    "session_id": "sess_...",
    "mfa": true
  },
  "delegation": {
    "actor": "ai_agent",
    "agent_role": "coder",
    "execution_id": "exe_...",
    "workspace_id": "ws_..."
  },
  "action": "deploy.start",
  "resource": {
    "kind": "project",
    "project_id": "prj_...",
    "tenant_id": "ten_...",
    "environment": "production"
  },
  "context": {
    "graphql_operation": "startDeploy",
    "request_ip_hash": "sha256:...",
    "data_residency": "us",
    "profitguard_decision_id": "pgd_...",
    "abuse_score": 31,
    "gate_state": {
      "security": "pass",
      "test": "pass",
      "deploy": "pass"
    }
  }
}
```

Required invariants:

- `tenant_id` must be present on principal, resource, execution, wallet,
  ledger entry, workspace, secret reference, audit row, and policy decision.
- Cross-tenant access is denied unless `principal.kind == platform_operator`
  and a break-glass approval is attached.
- AI delegation can reduce privileges but can never add privileges.
- Policy decisions are short lived; high-risk allows have single-use TTLs.

## Enforcement Points

| PEP | Must Ask Before |
| --- | --- |
| GraphQL handler | accepting mutations/subscriptions, introspection, persisted query registration, export queries |
| Resolver/domain service | creating execution, reading project, writing wallet/ledger-affecting state, querying dashboards |
| Provider router | sending tenant data to a model provider, switching provider, using high-cost reasoning |
| Runtime service | starting container, executing shell, network egress, filesystem write outside project root, package install |
| Patch engine | approving patch scope, touching security-sensitive files, applying generated diffs |
| Secret broker | injecting secret reference, rotating secret, binding secret to deploy provider |
| Deploy adapter | planning deploy, creating production deployment, promoting preview to production, rollback |
| Admin/operator tools | viewing tenant data, editing policy, replaying audit logs, break-glass actions |

## GraphQL Hardening

GraphQL is the API of record, so it is treated as a production attack surface,
not a convenience layer.

Required controls:

- Authentication on every operation except explicitly public infra probes, which
  stay outside GraphQL.
- Resolver-level tenant ownership checks for every object lookup.
- Max query depth, max query complexity, and per-field cost weights.
- Operation allowlist for production clients using persisted queries.
- Introspection disabled by default in production, enabled only for operator
  sessions or sandbox environments.
- Mutation and subscription rate limits keyed by tenant, user, session, IP hash,
  operation name, and abuse score bucket.
- Subscription fanout caps per tenant and per execution.
- No raw SQL filters, path selectors, provider prompts, or shell commands pass
  from GraphQL into backends without typed validation.
- Pagination is mandatory on all list fields; unbounded exports require explicit
  policy allow and audit.
- Error responses are redacted; internal IDs, provider errors, policy internals,
  and stack traces stay out of client-visible messages.
- CSRF protection for browser-authenticated GraphQL and strict origin checks for
  WebSocket upgrades.

GraphQL metrics must expose latency, complexity rejects, depth rejects, rate
limit rejects, policy denies, and operation-level error counts.

## Secret Handling

Secrets are capabilities, not strings the AI can see.

Rules:

- Store secret material only in the secret backend or provider vault; Postgres
  stores references, metadata, version IDs, and last-used timestamps only.
- Model context may contain secret names and intended use, never values.
- Workspaces receive scoped, time-limited secret mounts or environment bindings
  only after PDP allow and audit.
- Deploy providers receive secrets through provider-native secret APIs wherever
  possible; generated deployment files must reference secret names, not embed
  values.
- Secret reads are separated from secret writes. AI may propose a secret binding;
  a user or approved automation performs the write.
- Secret scanners run in the Security gate before patch apply and before deploy.
- Detected live credentials are critical findings: block deploy, rotate upstream,
  rewrite history where applicable, and record the remediation in audit.
- Logs, traces, model prompts, provider responses, GraphQL errors, and audit
  attrs use deterministic redaction before storage.

Allowed secret release classes:

| Class | Example | AI Visibility | Approval |
| --- | --- | --- | --- |
| `build_time_reference` | `STRIPE_SECRET_KEY` name | name only | tenant admin |
| `runtime_mount` | one deploy execution | value hidden | PDP + deploy approval |
| `operator_break_glass` | incident access | value hidden from AI | two-person approval |

## AI Command Policy

Runtime command execution is the sharpest edge of an AI builder. Ironflyer
therefore treats every command as a delegated, policy-governed side effect.

Default allowed command classes:

- Read-only inspection inside workspace root: `ls`, `find`, `rg`, `sed`, `cat`,
  `git diff`, language-native metadata commands.
- Deterministic builds/tests/lints declared by the project or blueprint:
  `go test`, `go vet`, `npm test`, `npm run build`, `npx tsc --noEmit`,
  framework-specific checks.
- Package-manager installs only when a lockfile or explicit blueprint step
  allows them, and only through approved registries.

Default denied command classes:

- Host access, privileged containers, Docker socket access, kernel modules,
  `sudo`, `su`, raw device access, or namespace escape attempts.
- Network egress to arbitrary hosts, tunneling, port scanning, crypto mining,
  scraping, credential harvesting, spam, or load testing third-party systems.
- Deleting history, wiping workspaces, disabling scanners, modifying audit
  plumbing, or changing policy bundles from inside an execution.
- Reading production databases, backups, secret stores, or other tenants'
  snapshots.
- Deploying or promoting production without deploy approval.

The runtime PEP records command hash, normalized argv, cwd, exit code, duration,
network class, touched path summary, execution ID, and policy decision ID. Raw
stdout/stderr are redacted before audit or model reuse.

## Deploy Approvals

Deploy is a privileged production action, never just the last step of codegen.

Deployment stages:

1. **Plan.** Generate deploy artifacts and provider plan. No production write.
2. **Preview.** Allowed only after Spec, UX, Arch, Code, Test, Security, and
   Deploy gates pass or have approved waivers.
3. **Production approval.** Requires an authenticated tenant admin or an
   automation policy with prior tenant consent.
4. **Promotion.** Deploy adapter performs the provider-side action with scoped
   credentials from the secret broker.
5. **Post-deploy verification.** Smoke check, ledger charge, audit chain entry,
   and rollback readiness are mandatory.

Production deploy requires:

- Passing Security and Deploy gates.
- ProfitGuard allow for deploy cost.
- PDP allow for `deploy.production.start`.
- Approval record with actor, tenant, project, environment, diff hash,
  artifact hashes, gate summary, and expiry.
- No critical abuse score or open policy deny on the execution.

Rollback is a privileged action too, but it may use an emergency policy with
short TTL when production health is failing.

## Production Data Isolation

AI workspaces are not production environments.

Controls:

- No direct production database credentials in workspaces or model prompts.
- Production snapshots are not mounted into sandboxes.
- Debug data for AI must be synthetic, anonymized, sampled with k-anonymity
  thresholds, or explicitly approved for that tenant and execution.
- Dataset exports are mediated by GraphQL export policy, tenant ownership, row
  limits, purpose, TTL, and audit.
- Provider routing must honor tenant data residency and data-processing policy.
- Error traces sent to models are scrubbed of secrets, personal data, auth
  tokens, payment data, and cross-tenant identifiers.
- Break-glass production access is operator-only, time-boxed, two-person
  approved, excluded from model context, and recorded in the audit chain.

If a task cannot be completed without production data, the execution pauses for
tenant approval and shows exactly what data class is requested and why.

## Audit Chain

The existing audit package is the trust spine: every consequential action
becomes a hash-chained record. V22 policy expands what must be recorded.

Mandatory audit events:

- Auth lifecycle and session changes.
- GraphQL high-risk mutations and denied operations.
- Policy decision allows for high-risk actions and all denies.
- ProfitGuard decisions.
- Provider dispatch with input/output hashes, model/provider ID, and redaction
  proof, not raw prompts.
- Workspace command execution.
- Secret reference writes, releases, rotations, and denied releases.
- Patch proposal, approval, apply, rollback, and verification.
- Gate verdicts and waivers.
- Deploy plan, approval, provider action, smoke result, rollback.
- Admin/operator break-glass.
- Abuse score escalations, throttles, suspensions, and manual overrides.

Audit integrity requirements:

- Content hash includes tenant, region, action, outcome, hashes, attrs, and
  previous hash.
- High-risk decisions include `policy_bundle_version` and `decision_id`.
- Long-term production retention uses Postgres plus WORM object storage or SIEM
  export; in-memory audit is development-only.
- `/audit/verify` remains a first-class operator and VSCode surface.
- Audit rows never contain secret values or unredacted model context.

## Abuse Scoring

Abuse scoring is not an afterthought rate limiter. It is input to policy.

Signals:

- Account age, payment state, chargeback/refund history, email/domain
  reputation, MFA state.
- GraphQL mutation velocity, subscription fanout, failed auth, policy denies,
  complexity/depth rejects.
- Prompt/command intent: malware, phishing, credential theft, spam, scraping,
  evasion, destructive ops, suspicious obfuscation.
- Runtime behavior: egress anomalies, package-install churn, scanner tampering,
  repeated command denies, CPU/network spikes.
- Provider behavior: safety refusals, prompt injection patterns, repeated
  attempts to exfiltrate secrets or production data.
- Economic anomalies: expensive retries with low completion delta, wallet abuse,
  provider-cost spikes, repeated failed deploys.

Policy thresholds:

| Score | Effect |
| --- | --- |
| `0-29` | Normal execution. |
| `30-59` | Tighter rate limits, more command review, high-cost provider downgrade. |
| `60-79` | Block risky commands, require manual approval for deploy/secret/network. |
| `80-100` | Pause execution, freeze deploy, require operator review. |

Abuse decisions are tenant-scoped, auditable, appealable by operators, and never
silently hidden from billing/support views.

## Tenant Isolation

Tenant isolation is enforced in every layer, not only by UI filters.

| Layer | Isolation Requirement |
| --- | --- |
| Identity | Every user session carries tenant membership and role claims. |
| GraphQL | Resolver checks ownership before returning any object. |
| Postgres | Every tenant-owned table has `tenant_id`; queries require tenant predicate. Row-level security is preferred for production. |
| Wallet/Ledger | Holds, debits, credits, and margin entries are tenant/execution scoped. |
| Runtime | Workspace IDs bind to tenant and execution; containers use per-tenant namespaces, cgroups, volumes, and network policy. |
| Object storage | Snapshots and exports use tenant-prefixed keys and tenant-scoped signed URLs. |
| Cache/Redis | Keys include tenant ID; pub/sub topics include tenant and execution. |
| Providers | Requests include tenant data policy; no cross-tenant prompt batching. |
| Audit | Query filters enforce tenant ownership; operator views require platform role and audit reason. |

Cross-tenant analytics are allowed only in explicitly operator-scoped dashboards
that aggregate or anonymize tenant data and record the access.

## Trust Differentiators vs Vibe Builders

Ironflyer competes on control and proof:

- Vibe builders generate files; Ironflyer ships through enforced gates.
- Vibe builders trust model behavior; Ironflyer constrains delegated AI with a
  policy plane and audited side effects.
- Vibe builders hide cost until later; Ironflyer routes every expensive step
  through wallet, ledger, and ProfitGuard.
- Vibe builders blur dev and prod; Ironflyer separates workspace execution,
  secret release, deploy approval, and production data access.
- Vibe builders show chat logs; Ironflyer shows verifiable decisions, hashes,
  patch history, gate verdicts, and deploy evidence.

The market promise is simple: speed with receipts.

## V22 Acceptance Checklist

- [ ] OPA PDP service/library wired into GraphQL, runtime, deploy, secret broker,
      provider router, and patch engine.
- [ ] Cedar-compatible app authorization model defined for user, tenant,
      project, execution, workspace, wallet, secret, and deploy resources.
- [ ] Policy bundle signing and versioning in CI/CD.
- [ ] Policy decision table and audit events include `decision_id`,
      `policy_bundle_version`, effect, reason, obligations, and TTL.
- [ ] GraphQL production hardening enabled: persisted queries, depth/complexity
      caps, resolver ownership checks, rate limits, redacted errors.
- [ ] AI command policy enforced by runtime before command execution.
- [ ] Secret broker blocks raw secret exposure to model context and workspaces.
- [ ] Deploy approvals require gate summary, artifact hashes, diff hash, actor,
      environment, and expiry.
- [ ] Production data access is denied to AI by default and mediated by explicit
      tenant approval when unavoidable.
- [ ] Abuse score feeds PDP and can pause executions.
- [ ] Tenant isolation is testable at GraphQL, store, runtime, object storage,
      cache, provider, audit, and dashboard layers.
