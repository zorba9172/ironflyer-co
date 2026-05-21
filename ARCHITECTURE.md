# Ironflyer Architecture — Locked Spec

> Ironflyer is an **AI Product Finisher**, not a chatbot, not a no-code toy,
> not a VSCode clone.

## Locked Decisions

1. **Core language**: Go (orchestrator, runtime, inference, sandbox, patch).
2. **Web**: Next.js 15 + MUI 6 + React 19.
3. **Design**: output.com aesthetic, lovable.dev/dashboard flow.
4. **No heavy IDE embedded**. code-server inside per-user workspace only.
5. **AI never mutates files directly**. Patch lifecycle is mandatory.
6. **Finisher Gates** are the differentiator. Spec/UX/Arch/Code/Test/Sec/Deploy.
7. **Multi-provider** by capability + cost + tenant policy.
8. **Private AI** (ONNX) for intent, embeddings, PII, scoring.
9. **Mobile takeaway** via PWA, server-authoritative.

## Monorepo Layout

```
ironflyer/
├── apps/
│   ├── api/                # Go API gateway (auth, rate limit, tenant)
│   ├── orchestrator/       # Go finisher engine (DAG + gates + repair loop)
│   ├── inference/          # Go ONNX private AI service
│   ├── runtime/            # Go workspace runtime (containers, FS, PTY)
│   ├── web/                # Next.js + MUI dashboard (output.com aesthetic)
│   └── mobile/             # PWA shell (reuses web bundle)
├── packages/
│   ├── agents/             # Agent prompt templates + JSON schemas
│   ├── design-tokens/      # output.com-inspired tokens
│   ├── ui/                 # Shared MUI components
│   └── sdk/                # Client SDK (TS)
├── services/
│   ├── figma-slicer/       # Go service for Figma → component tree → code
│   ├── patch-engine/       # Go patch validate/apply/rollback
│   └── sandbox/            # Container runner (Docker → Firecracker)
├── infra/
│   ├── compose/            # docker-compose.dev.yml
│   ├── docker/             # Dockerfiles per service
│   └── k8s/                # Future k8s manifests
└── docs/
```

## Core Loop

```
Idea
  → Brainstorm (only when needed; multi-model bake-off)
  → Product Spec
  → UX Flow
  → Figma slicing
  → Architecture
  → Code Generation
  → Validation
  → Debugging (auto-repair)
  → Security Review
  → Deployment Readiness
  → Final Product Package
```

## Finisher Gates

| Gate     | Definition of done                                                                 | Repair agent |
| -------- | ---------------------------------------------------------------------------------- | ------------ |
| Spec     | user stories + acceptance criteria + data model sketch                             | Planner      |
| UX       | every story has a screen + state diagram                                           | UXer         |
| Arch     | services/data/contracts mapped, no dangling deps                                   | Architect    |
| Code     | builds, types clean, all routes implemented, error states present                  | Coder        |
| Test     | unit + e2e cover happy + 2 edge paths per story                                    | Tester       |
| Security | OWASP top-10 scan clean, secrets scan clean, authz check                           | Sec          |
| Deploy   | Dockerfile, env documented, healthcheck, runbook                                   | Deployer     |

Every gate implements `Check(project) []Issue`. Failed gates dispatch a
targeted repair task to the matching agent. The loop terminates only when all
gates pass, max iterations is hit, or the user intervenes.

## Agent Runtime Model

- Stateless functions with typed input/output JSON Schemas.
- Orchestrator owns state. Agents are pure.
- Every call routed through Provider Router (capability + cost + policy).
- All outputs validated against schema. Retry-with-feedback on failure.

## Provider Router

- **Capability tags** per request: `reasoning`, `code`, `json`, `vision`,
  `cheap`, `fast`, `private`.
- **Per-tenant policy**: cost cap, preferred provider, data residency.
- **Fallback chain** on provider error.
- **Private/ONNX** for: intent classification, embeddings, PII redaction,
  quality scoring (kept local for privacy + latency).

## Patch Lifecycle

```
propose → validate(syntax, types, security, scope)
        → preview diff
        → approve (auto if low-risk policy met, manual otherwise)
        → apply (atomic, transactional FS)
        → snapshot (git-backed)
        → verify (tests + gates)
        → rollback on verification failure
```

## Workspace

- Phase 1: Docker rootless container per user, code-server (lite) inside,
  overlay FS (base image + per-user diff), PTY exposed via WS to xterm.js.
- Phase 2: Firecracker microVMs for stronger isolation.
- All file operations go through the runtime FS API (audit + versioning).

## Mobile Takeaway

- PWA installable.
- Server-authoritative: session resumable on any device.
- Mobile mode: read code, chat-driven edits, approve patches, deploy.
- Push notifications on completion / blocker.

## Communication

- REST for CRUD.
- SSE for streaming agent output.
- WebSocket for workspace events (file changes, PTY).
- gRPC between Go services.

## Storage

- **Postgres** — projects, users, gates, patches, billing.
- **Redis** — job queue, ephemeral state, SSE fan-out.
- **MinIO** (S3-compatible) — snapshots, exports, Figma assets.
- **pgvector** — code/spec embeddings.

## Observability

- OpenTelemetry traces across all Go services.
- Per-request agent trace tree (planner → coder → reviewer).
- Cost ledger per provider per user.
- Quality scorecard per project (gate history).
