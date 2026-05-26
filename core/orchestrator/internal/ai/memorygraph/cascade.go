package memorygraph

// Agent 30 — project-deletion cascade for the AI Memory Graph.
//
// The projects resolver (and any future admin tool that calls
// store.Store.Delete on a project) used to leave every Node / Edge
// the finisher had projected for that project sitting in the graph
// forever. Two problems with that:
//
//  1. Stale recall. After a user deletes a project and creates a
//     fresh one under the same name, the IntentGateRepair traversal
//     could surface symbols / failures / repairs from the dead
//     project as "context" for the new one.
//  2. Retention drift. SurrealDB / pgvector backends accumulate dead
//     rows the operator dashboard has no way to clean up short of a
//     manual SurrealQL purge.
//
// CascadeProjectDeletion is the single bottleneck callers route
// through. It's deliberately tiny — the heavy lifting is already done
// by Graph.DeleteProject (Agent 18). This wrapper exists so the
// projects resolver doesn't need to know about graph internals AND so
// follow-up cleanup (cache busts, projection-consumer rewinds) has one
// place to land later.

import (
	"context"
	"fmt"
)

// CascadeProjectDeletion removes every node + edge owned by projectID
// inside tenantID. Returns nil when graph is nil (the orchestrator may
// boot without a graph backend wired) so callers don't have to nil-
// check on every delete path.
//
// The contract is best-effort: callers SHOULD invoke this in a
// goroutine after the canonical project delete commits, so a slow or
// flaky graph backend cannot fail the user-visible delete RPC.
// CascadeProjectDeletion itself never spawns goroutines — that policy
// belongs to the caller.
func CascadeProjectDeletion(ctx context.Context, g Graph, tenantID, projectID string) error {
	if g == nil {
		return nil
	}
	if tenantID == "" || projectID == "" {
		return fmt.Errorf("memorygraph: cascade requires tenantID + projectID")
	}
	if err := g.DeleteProject(ctx, tenantID, projectID); err != nil {
		return fmt.Errorf("memorygraph: cascade delete project %s/%s: %w", tenantID, projectID, err)
	}
	return nil
}
