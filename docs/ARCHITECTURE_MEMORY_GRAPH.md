# Ironflyer Architecture - SurrealDB AI Memory Graph

This document defines the single V22 role for SurrealDB: the **AI Memory
Graph**. It is the graph and retrieval layer that helps agents understand
projects, code, previous patches, and repeated failures. It is not an
economic ledger and it is not the execution source of truth.

## Purpose

SurrealDB stores derived, queryable memory for the finisher loop:

- **Project graph**: project goals, specs, gates, milestones, decisions, and
  their dependency edges.
- **Code graph**: repositories, files, symbols, imports, ownership zones,
  generated artifacts, and validation surfaces.
- **Agent memory**: bounded summaries of prior agent attempts, useful
  observations, rejected approaches, and successful context packets.
- **Repair genome context**: failure signatures, fix recipes, related files,
  known-good patches, verification outcomes, and recurrence signals.
- **Vector and hybrid retrieval**: embeddings plus structured graph filters so
  the orchestrator can retrieve context by meaning, by relation, or both.
- **Spec-to-change lineage**: explicit relations from specs to files to
  patches to failures to repairs.

The graph exists to lower cost and increase completion rate by giving agents
the right context before they spend tokens, touch a sandbox, or repeat a known
repair path.

## Non-Goals

SurrealDB must not store or arbitrate V22 economic or execution truth:

- No wallet balances, holds, credits, debits, or payment state.
- No append-only ledger rows, provider charges, sandbox charges, platform
  margin, or reconciliation state.
- No canonical execution FSM state, admission result, commit result, or
  completion score.
- No ProfitGuard audit truth. It may link to ProfitGuard decision IDs for
  retrieval, but the decisions themselves remain in Postgres.
- No artifact truth. Snapshots and exports remain in object storage; the graph
  stores references and derived summaries only.

If losing SurrealDB would change a user's balance, hide a billable charge,
rewrite an execution result, or make an audit trail incomplete, that data is in
the wrong store.

## Ownership Boundary

| Concern | Canonical store | SurrealDB role |
| --- | --- | --- |
| Wallet balance and holds | Postgres | No copy |
| Ledger entries | Postgres append-only ledger | No copy |
| Execution state | Postgres execution tables | Links by execution ID only |
| ProfitGuard decisions | Postgres audit rows | Links and retrieval metadata only |
| Projects and specs | Postgres for canonical project records | Derived graph nodes and edges |
| Patches | Postgres/runtime snapshot references | Patch intent, summary, affected graph edges |
| Repair recipes | Postgres repair tables | Retrieval context and recurrence relations |
| Source files | Runtime/git snapshot/object storage | Symbol/file graph and summaries |
| Embeddings | SurrealDB memory graph | Native vector/hybrid retrieval surface |

SurrealDB data is rebuildable from canonical project, patch, repair, execution,
and snapshot events. Rebuildability is the guardrail that keeps the graph from
becoming a second truth system.

## Graph Shape

The minimum graph vocabulary is intentionally small:

### Nodes

- `project`: derived project context keyed by canonical project ID.
- `spec`: product, UX, architecture, security, deploy, and acceptance specs.
- `gate`: finisher gate instance and current derived context.
- `file`: file path at a snapshot, including ownership and language metadata.
- `symbol`: package, type, function, component, route, resolver, migration, or
  schema element.
- `patch`: patch proposal/application summary keyed by canonical patch ID.
- `failure`: test, gate, build, security, deploy, or runtime failure signature.
- `repair`: repair genome context keyed by canonical repair recipe ID.
- `agent_memory`: compact observation or attempt summary with TTL/classification.
- `embedding_chunk`: retrievable text/code/spec chunk with vector metadata.

### Relations

- `project -> has_spec -> spec`
- `spec -> requires_gate -> gate`
- `spec -> concerns_file -> file`
- `file -> defines_symbol -> symbol`
- `symbol -> imports|calls|renders|queries -> symbol`
- `patch -> modifies_file -> file`
- `patch -> implements_spec -> spec`
- `patch -> caused_failure -> failure`
- `failure -> observed_in_file -> file`
- `failure -> matches_repair -> repair`
- `repair -> fixed_by_patch -> patch`
- `agent_memory -> about_project|about_file|about_failure|about_repair -> node`

The high-value traversal for V22 is:

```
spec -> files -> patches -> failures -> repairs -> known-good patches
```

That traversal powers `reuse_repair`, scoped code context, and "do not repeat
this failed approach" prompts.

## Retrieval Contract

The orchestrator asks SurrealDB for **context packets**, not raw global memory.
A context packet has:

- scope: tenant ID, project ID, optional execution ID, and allowed file paths.
- intent: gate repair, code generation, review, security, deploy, or completion
  scoring support.
- retrieval mode: graph traversal, vector similarity, keyword filter, or hybrid.
- budget: maximum records, token estimate, freshness window, and confidence
  threshold.
- provenance: canonical IDs for every project/spec/file/patch/failure/repair
  reference included.

Agents receive only the context packet. They do not query SurrealDB directly.
The orchestrator remains responsible for tenant isolation, scope checks,
budgeting, and ProfitGuard enforcement before expensive retrieval.

## Write Contract

Only orchestrator-owned event handlers write to the memory graph. Writes happen
after canonical state is recorded elsewhere:

1. A spec, gate, patch, failure, repair, or snapshot event lands in its
   canonical store.
2. The graph writer projects that event into nodes, edges, summaries, and
   embedding chunks.
3. Retrieval indexes are updated asynchronously.
4. Failures to update SurrealDB are observable but do not invalidate the
   canonical execution.

Graph writes are idempotent by canonical IDs. Reprocessing the same execution
events must converge on the same graph.

## Repair Genome Context

The repair genome remains the product concept: a failure signature maps to a
known fix recipe that reduces repeat repair cost. SurrealDB is the context
layer around that genome:

- find similar failures across the same project, blueprint, framework, or file
  family.
- connect failures to patches that caused or fixed them.
- rank repairs by recent success, affected files, and gate type.
- retrieve nearby code/spec context for the repair agent.

The canonical recipe, hit count, and economic outcome remain outside
SurrealDB. The graph may cache success summaries and relation strength, but
wallet, ledger, execution, and margin numbers remain in Postgres.

## Operational Rules

- Tenant and project isolation are mandatory on every query and write.
- Raw prompts, secrets, credentials, payment metadata, and full ledger payloads
  are never stored in the graph.
- Agent memory is compacted and classified; long chat transcripts are not a
  graph primitive.
- Edges must carry provenance, timestamp, source event ID, and confidence.
- Low-confidence inferred edges are marked as inferred and are never promoted
  to canonical state without a canonical event.
- Deleting a project deletes or tombstones its graph projection according to
  the retention policy, without touching ledger or execution audit records.

## Backend Position

SurrealDB is the canonical V22 backend for AI memory graph behavior. Any
in-process memory or pgvector mode is a development or compatibility fallback
for simple memory search; it is not the architectural owner for graph
relations, repair-genome context, or hybrid retrieval.
