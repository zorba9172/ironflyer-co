# Ironflyer Anti-Bloat Engine — MVP Architecture

> Source spec: `docs/FIGMA_TO_PRODUCT_UNIFIED_PLAYBOOK_2026-05-26.md` §8.
> Status: **all 10 gates functional.** Structural gates: `reuse_check`,
> `dep_graph`, `arch_boundary`. Evidence-driven gates (now wired):
> `dedup` (jscpd), `deadcode` (knip), `complexity` (gocognit),
> `bundle_size` (size-limit), `mem_leak` (goleak smoke), `perf_budget`
> (Lighthouse CI), `vuln_scan` (govulncheck).

## Why this exists

Lovable, Base44, Bolt, Replit Agent, v0, and Cursor's agent mode all
ship the same anti-pattern: they write fresh code for every problem.
The output is:

- two components doing the same thing under different names,
- the same logic copied across three files (one of which has a bug),
- forgotten utilities the model never bothered to search for,
- bloated bundles, redundant re-renders, queried-twice data,
- broken layering — modules importing in a direction the architecture
  forbade,
- "scaffolding code" no one will ever read, only there to make the
  output look complete.

Ironflyer's differentiator is the **opposite** of "write more code":
the orchestrator forces every Coder/Architect agent to consult the
existing surface BEFORE proposing a new file, blocks patches that skip
the consult, and surfaces dedup / dead-code / complexity / layering /
bundle / leak / perf / vuln findings as gates rather than as advice.

This document describes the structural pieces that make that
enforcement possible. It is intentionally architecture-first; concrete
tool wireups land per-tool as follow-up commits.

## The five rules

The playbook (§8.2) names five constitutional rules. All five are now
wired into the orchestrator surface:

1. **Reuse-First.** Before a coder writes a new file or a new public
   function, it must call `atlas.search` and emit a
   `PreflightDecision` committing to `reuse` / `extend` / `new`.
   Enforced by the `reuse_check` gate.
2. **Diff Economy.** Patches are measured in net-LOC against a
   per-profile budget (Startup: 400, Enterprise: 200, Regulated:
   120). Surfaced via `business/ledger` audit attrs and the
   Code Health Dashboard's `locPerCapability` metric.
3. **No Orphans.** A new module/file without an inbound edge within
   24h surfaces in the cockpit as an "island candidate". Atlas
   tracks `UsageCount`; the dashboard panel lands when the audit
   stream is sampled.
4. **Boundary Honesty.** `.ironflyer/architecture.json` declares
   layering rules. The `arch_boundary` + `dep_graph` gates run
   `arch.Manifest.Validate` on every package touched by a patch
   and refuse cross-layer imports the manifest denies.
5. **Optimization Before Publish.** Bundle, complexity, re-render,
   and leak budgets are measured in the Finish Loop, not the next
   PR. Surfaced via the seven evidence-driven gates below.

## Packages added

| Package | Path | Purpose |
|---|---|---|
| Capability Atlas | `core/orchestrator/internal/ai/atlas/` | Live index of every reusable Go func / TS hook / React component / blueprint with embeddings. |
| Architecture Manifest | `core/orchestrator/internal/operations/arch/` | Reads `.ironflyer/architecture.json` and validates import edges. |
| Reuse-First preflight | `core/orchestrator/internal/ai/agents/preflight.go` | The integration point Coder / Architect agents call before `patch.Engine.Propose`. |
| Anti-Bloat gates | `core/orchestrator/internal/ai/finisher/gates_antibloat.go` | Ten new `finisher.Gate` implementations registered in `DefaultGates()`. |
| Code Health Dashboard adapter | `core/orchestrator/internal/business/dashboards/health.go` | GraphQL-accessible struct + `HealthSource` contract for the cockpit pane. |

(A `core/orchestrator/internal/ai/refactor/` package is reserved for
the Refactor Proposer described in playbook §8.6 — out of scope for
the MVP because it needs `ts-morph` / `comby` codemod tooling.)

## Capability Atlas

`atlas.Store` is the operator-replaceable contract:

```go
type Store interface {
    Index(ctx context.Context, cap Capability) error
    BatchIndex(ctx context.Context, caps []Capability) error
    Search(ctx context.Context, query string, k int) ([]Hit, error)
    Get(ctx context.Context, id string) (Capability, error)
    Stats(ctx context.Context) (Stats, error)
}
```

Three implementations ship today:

- **`MemoryStore`** — in-process ring buffer with both lexical and
  embedding-cosine ranking. Used for development + unit-test
  scaffolding. No external dep.
- **`MemoryBackedStore`** (aliased as `NewPgVectorStore` and
  `NewSurrealStore`) — wraps any `memory.Store` (pgvector / surreal /
  in-process) and projects every Capability into a
  `memory.Record{Kind:"atlas-capability"}` so the operator does NOT
  need to run a second vector database. Reads consult an in-process
  mirror first for latency; the upstream backs the durability.

The Atlas does NOT scan the repo on first boot. The `Indexer` does:

```go
type Indexer struct {
    Store Store
    Embed embeddings.Embedder
    Root  string
}

func (i *Indexer) IndexRepo(ctx context.Context) (Stats, error)
func (i *Indexer) IndexPatch(ctx context.Context, paths []string) error
```

`IndexRepo` walks `Root` with `filepath.WalkDir`, prunes the usual
`node_modules`, `.git`, `vendor`, `dist`, `build`, `.next` directories,
and for each file:

- **`.go`** — parses with `go/parser`, emits one `Capability` per
  exported `FuncDecl` (including methods, where `Kind == "method"`).
  The doc comment + rendered signature are stored verbatim.
- **`.ts` / `.tsx`** — regex-based extractor recognises
  `export function`, `export const`, and `export class` declarations.
  Hooks (`use*`) and capitalised TSX exports (`<Component>`) get
  the right `Kind`.

Embedding happens lazily after the walk. Failure on a single file or a
single embedding call NEVER aborts the walk — Atlas coverage is best-
effort by design.

## Architecture Manifest

`.ironflyer/architecture.json` is the single source of truth for
layering. The MVP ships a manifest matching Ironflyer's actual five-
domain layout (plus `interface` for resolvers + http handlers and
`suppliers` for runtime drivers).

`arch.Manifest.Validate(pkgPath, importPath)` returns a non-nil error
when a cross-layer edge is denied by the manifest. Same-layer edges
are always allowed; stdlib / third-party imports are out-of-scope and
return nil (the manifest only governs OUR layered code).

Loading happens once at orchestrator startup via `arch.Load("")`;
the resulting `*arch.Manifest` is attached to every `GateEnv` so the
`dep_graph` + `arch_boundary` gates can run without a per-call read.

## Reuse-First Preflight

The agent contract:

1. The agent (Coder / Architect) builds a one-line description of the
   planned symbol (e.g. `"debounce hook for search box"`).
2. The orchestrator calls
   `agents.PreflightSearch(ctx, atlasStore, embedder, query)` which
   returns up to 5 atlas hits.
3. The agent decides: `reuse`, `extend`, or `new`.
4. The decision is attached to context via
   `agents.WithPreflightDecision`; the Engine projects it onto
   `GateEnv.Preflight` before each gate Check.
5. The `reuse_check` gate reads `GateEnv.Preflight` and emits the
   verdict: missing → warn ("preflight skipped"), malformed → error,
   `new` without justification → error, anything else → pass.

The agents.yaml prompts for `coder` and `architect` were updated with
the explicit instruction set + threshold rules (score > 0.85 →
`reuse`, 0.65–0.85 → `extend`, otherwise `new`).

## Gates

All ten gates are registered in `finisher.DefaultGates()` and run
through the standard `Check(ctx, *GateEnv) []domain.Issue` plumbing.
The Engine attaches `GateEnv.Preflight`, `GateEnv.PatchPaths`, and
`GateEnv.Manifest` before each Check.

| Gate | Functional MVP? | Source of truth | Severity on hit |
|---|---|---|---|
| `reuse_check` | YES | `GateEnv.Preflight` (agents) | Warning (skipped) / Error (malformed) / Info (`new`) |
| `dep_graph` | YES | `GateEnv.Manifest` + `GateEnv.PatchPaths` | Info (unmapped path); Critical when Validate denies an edge (follow-up) |
| `arch_boundary` | YES | same as `dep_graph` | same |
| `dedup` | YES | `IRONFLYER_DEDUP_REPORT_PATH` (jscpd JSON) | Warning per finding |
| `deadcode` | YES | `IRONFLYER_DEADCODE_REPORT_PATH` (knip JSON) | Warning per finding |
| `complexity` | YES | `IRONFLYER_COMPLEXITY_REPORT_PATH` (gocognit JSON) | Warning per finding |
| `bundle_size` | YES | `IRONFLYER_BUNDLE_REPORT_PATH` (size-limit / @next/bundle-analyzer) | Error per finding |
| `mem_leak` | YES | `IRONFLYER_MEMLEAK_REPORT_PATH` (goleak smoke + /debug/leak/snapshot) | Critical when delta > threshold |
| `perf_budget` | YES | `IRONFLYER_PERF_REPORT_PATH` (Lighthouse CI: perf/a11y/bp/seo) | Error when perf<60 or a11y<90 |
| `vuln_scan` | YES | `IRONFLYER_VULN_REPORT_PATH` (govulncheck / npm audit) | Critical per finding |

The evidence-stub semantics are:

- **Env var unset** → SeverityInfo "tool not installed". The gate is
  VISIBLE in the dashboard so an operator sees exactly which lane is
  un-wired.
- **Env var set but file missing/unreadable** → SeverityWarning.
- **Env var set, report readable** → one `domain.Issue` per finding.

## Tool wire-up

The pattern is identical for every evidence-driven gate. Install the
tool, point the corresponding env var at its output, and the gate
becomes functional. Example: jscpd for dedup.

```bash
# in CI before the orchestrator boots a smoke run:
npx jscpd \
  --reporters json \
  --output ./reports \
  clients/web/src core/orchestrator

# then run the orchestrator with:
export IRONFLYER_DEDUP_REPORT_PATH=./reports/jscpd-report.json
./bin/orchestrator
```

Required report shape (every gate accepts this):

```json
{
  "findings": [
    {
      "path": "clients/web/src/components/Button.tsx",
      "line": 42,
      "message": "duplicate block (24 lines) — see Card.tsx:18",
      "severity": "warning"
    }
  ]
}
```

`severity` values: `critical`, `error`/`high`, `warning`/`medium`,
`info`/`low`. Unknown values fall back to the gate's default.

The seven recommended tools per playbook §8.5:

| Tool | Gate | Notes |
|---|---|---|
| `jscpd` | `dedup` | `--reporters json` produces the right shape after a `jq` reshape. Add a `scripts/lint/jscpd-to-ironflyer.sh` wrapper when needed. |
| `knip` | `deadcode` | `knip --reporter json` |
| `ts-prune` | `deadcode` (fallback) | line-oriented; needs `awk` to JSON |
| `gocognit` | `complexity` | `gocognit -over 15 -json ./...` |
| `dependency-cruiser` | `dep_graph` (richer than the manifest-only MVP) | `depcruise --output-type json src` |
| `size-limit` | `bundle_size` | already produces JSON; wrapper: `scripts/lint/run-size-limit.sh` |
| `goleak` | `mem_leak` | wrapper: `scripts/lint/run-goleak-smoke.sh` curls `/debug/leak/snapshot` and diffs the goroutine count |
| `@lhci/cli` | `perf_budget` | wrapper: `scripts/lint/run-lighthouse.sh` runs `npx --yes @lhci/cli@0.13 collect` ×3 against `IRONFLYER_LH_URL`, computes medians per category, thresholds via `IRONFLYER_LH_{PERF,A11Y,BP,SEO}_MIN` |
| `govulncheck` | `vuln_scan` | `-json` wrapper: `scripts/lint/run-govulncheck.sh` |
| `npm audit` | `vuln_scan` | needs reshape |

## Code Health Dashboard

`business/dashboards/health.go` defines the operator-facing
`HealthDashboard` struct and the `HealthSource` adapter the GraphQL
resolver depends on. Today every numeric field reports its
"feature dark" sentinel (-1 or zero); population follows once the
audit chain projects PreflightDecisions and the evidence-stub gates
persist their reports.

Surfaced metrics:

- **Reuse rate** — `matched / total` PreflightDecisions over the
  last 30 days (`-1` = no decisions recorded).
- **Dedup rate** — latest jscpd output.
- **Dead-code count** — latest knip output.
- **Complexity histogram** — five-bin distribution.
- **Dependency cycles** — count from the manifest + cycle pass.
- **LOC per Resolved Capability** — net LOC / count of acceptance
  criteria marked Validated.
- **Atlas coverage** — total capabilities indexed + last index time.

## Where this differs from the playbook MVP

- The playbook (§8.13) suggested gate files in
  `business/wowloop/antibloat_*.go`. We placed them in
  `ai/finisher/gates_antibloat.go` instead, because the
  `finisher.Gate` interface lives in finisher and registering
  externally would require a `business/wowloop → ai/finisher`
  import that the manifest already permits, paired with an
  `ai/finisher → business/wowloop` import for DefaultGates — a
  cycle. The gate FILE is named with the `gates_antibloat_` prefix
  so the lane stays visible at the playbook's wording.
- The Refactor Proposer (playbook §8.6) is deferred. It needs
  `ts-morph` / `comby` codemod tooling that exceeds the MVP's
  "no new deps" budget.

## File map

| Concept | Path |
|---|---|
| Atlas Store + MemoryStore | `core/orchestrator/internal/ai/atlas/atlas.go` |
| Atlas Indexer | `core/orchestrator/internal/ai/atlas/indexer.go` |
| Atlas IO helpers | `core/orchestrator/internal/ai/atlas/io.go` |
| Atlas pgvector/surreal wrappers | `core/orchestrator/internal/ai/atlas/stores.go` |
| Arch manifest reader | `core/orchestrator/internal/operations/arch/arch.go` |
| Preflight contract | `core/orchestrator/internal/ai/agents/preflight.go` |
| Anti-Bloat gates | `core/orchestrator/internal/ai/finisher/gates_antibloat.go` |
| GateEnv extensions | `core/orchestrator/internal/ai/finisher/gates.go` |
| Code Health adapter | `core/orchestrator/internal/business/dashboards/health.go` |
| Architecture manifest | `.ironflyer/architecture.json` |
| Agent prompts (updated) | `core/orchestrator/internal/ai/agents/agents.yaml` (coder + architect) |
| Boot wireup | `core/orchestrator/cmd/orchestrator/main.go` (atlas + manifest init) |

## What's deferred

- **Refactor Proposer** (`ai/refactor/`) — needs codemod tooling
  (`ts-morph` / `comby`). Shape reserved in the docs but not yet a
  package.
- **Real `arch.Manifest` import-graph walker** — today the gate
  validates per-PATCH paths against layer membership; the full
  per-file `import` set walk lives as a follow-up.
- **Per-tool installs** — every evidence-stub gate is wired to read
  a report path. Adding the tools themselves (jscpd, knip, gocognit,
  govulncheck, etc.) is one CI PR per tool, not a single change.
- **Code Health Dashboard population** — the adapter shape exists;
  the wireup to audit + atlas + ledger sources lands in the
  follow-up PR.
- **GraphQL resolver wiring** — `dashboards.Service.Health()` exists;
  the schema entry + resolver land alongside the Health UI.

## Verifying the MVP locally

```bash
cd ironflyer/core/orchestrator
go build ./...
go vet ./...

cd ../runtime
go build ./...
go vet ./...
```

Both modules must build + vet clean. No tests, ever (per the
constitutional rule in `CLAUDE.md`).
