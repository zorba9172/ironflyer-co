# CLI ↔ GraphQL gaps

The CLI's transport layer is now genqlient-powered (`core/cli/internal/gql`).
Most surfaces map cleanly; the items below either reuse REST intentionally
or work around schema fields that haven't been added yet.

## REST that stays REST forever

These mirror the orchestrator's exception list in `CLAUDE.md` (k8s
probes etc) — they MUST NOT be GraphQL-wrapped.

- `Client.Health(ctx)` / `Client.HealthAt(ctx, base)` hit `GET /healthz`
  (then fall back to `/health`). Used by `ironflyer status` to ping both
  the orchestrator and the runtime. The runtime does not serve a
  GraphQL endpoint, and probes are explicitly carved out of the
  REST sunset.
- `Client.ExportZip` resolves `mutation exportZipUrl` and then GETs the
  returned signed URL to stream the archive. The download itself is in
  the REST exception list (`/audit/export.*` shows the same pattern).

## Schema fields not yet exposed (CLI works around)

- **`Project.gates`** — `domain.Project` carries a gate-state map but
  `core/orchestrator/internal/graph/schema/projects.graphql` does not
  expose it. `ironflyer projects show` renders an empty gates table
  until a `gates: [GateState!]!` field is added. The `GateState` Go
  type is retained in `internal/client/client.go` so adding the
  resolver later is one struct populate.
  TODO(graphql): expose gate state on the Project type.

- **`startDeploy(provider, region, env)`** — REST took `(provider,
  region, env)` as discrete fields. The GraphQL `StartDeployInput`
  schema only carries `target` (≈ provider) and an opaque `env: JSON`.
  The CLI packs `region` into the env JSON under `__region__` so the
  orchestrator's deploy engine can still read it.
  TODO(graphql): add `region: String` to `StartDeployInput`.

- **`exportGithub(owner, repo)`** — REST took `(repoName, description,
  private)`. The GraphQL surface only has `(owner, repo)`. The CLI
  splits the `--repo-name` flag on `/`, falling back to the calling
  user's email prefix as the owner. `--description` and `--private`
  are accepted but unused server-side until the schema grows fields.
  TODO(graphql): add `description: String`, `private: Boolean`, and
  return a `GithubLink!` instead of `OperationResult`.

- **`AuditEntry`** — REST returned a flat row with `summary`,
  `inputHash`, `outputHash`, `agentRole`, etc. The GraphQL schema
  collapses everything except (id, ts, userId, projectId, action,
  outcome, hash, prevHash) into the `payload: JSON` blob. The CLI
  re-extracts `summary`, `gateName`, `storyId`, `agentRole` from the
  payload when present and surfaces the rest under `attrs`.
  TODO(graphql): consider promoting first-class fields back onto
  `AuditEntry` so consumers don't need to peek inside `payload`.

- **`AgentTelemetry` cache token counters** — REST exposed
  `cacheReadTokens` and `cacheNewTokens`; the GraphQL `AgentCall` type
  only has `promptTokens` and `completionTokens`. The CLI populates
  the cache counters as zero.
  TODO(graphql): add `cacheReadTokens: Int`, `cacheNewTokens: Int` to
  `AgentCall`.

## Subscription mapping

The CLI keeps its `SSEEvent { Event, Data }` shape so command code in
`core/cli/internal/commands/{run,deploy}.go` did not have to change.
Each typed event from `runProject` / `deployStream` is JSON-encoded
back into a flat map with the keys the existing renderers look for
(`role`, `message`, `gate`, `status`, `kind`, `line`, `url`, …).

Transport: `github.com/coder/websocket` (already vendored by
`core/runtime`) adapted to genqlient's `Dialer` / `WSConn` interfaces.
Subprotocol: `graphql-transport-ws`. Auth: `connection_init` carries
`{"authorization": "Bearer <token>"}` in its payload, plus a redundant
`Authorization` HTTP header on the upgrade request itself.
