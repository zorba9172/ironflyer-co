// Package memorygraph implements the V22 AI Memory Graph: derived,
// queryable memory that helps agents understand projects, code, prior
// patches, repeated failures, and known-good repairs. See
// docs/ARCHITECTURE_MEMORY_GRAPH.md for the contract.
package memorygraph

import "errors"

// ErrNotFound is returned by Graph.GetNode and similar reads when no
// matching record exists. Surfaces as a "missing" signal to callers;
// retrieval paths SHOULD treat it as an empty result, not a hard error.
var ErrNotFound = errors.New("memorygraph: node not found")

// ErrTenantMissing is returned when a write or query lacks a tenant id.
// Tenant isolation is mandatory on every Graph method (see
// docs/ARCHITECTURE_MEMORY_GRAPH.md "Operational Rules").
var ErrTenantMissing = errors.New("memorygraph: tenant id required")

// ErrProjectMissing is returned when a per-project method is invoked
// without a project id. Project scoping is mandatory for every
// projection that descends from a project (specs, files, patches,
// failures, repairs, agent_memory).
var ErrProjectMissing = errors.New("memorygraph: project id required")

// ErrInvalidNode is returned when a node fails minimum-shape validation
// (missing kind, empty id components, etc.). Idempotent upserts MUST
// reject malformed payloads early so the graph never ingests partial
// state.
var ErrInvalidNode = errors.New("memorygraph: invalid node")

// ErrInvalidEdge is returned when an edge references unknown node ids,
// has a zero kind, or violates the per-kind from/to type contract.
var ErrInvalidEdge = errors.New("memorygraph: invalid edge")
