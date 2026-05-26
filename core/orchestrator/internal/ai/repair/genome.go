package repair

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Recipe is one entry in the repair genome.
//
//   - FailureSignature is the SHA-256 hex of the normalised failure
//     string (see signature.go). It is the unique key.
//   - Category is the failure family (e.g. "lint", "type", "test",
//     "build", "deploy"). Used for analytics and routing.
//   - Fix is the structured fix recipe (free-form JSON). The finisher
//     interprets the shape.
//   - Hits is the running count of lookups that matched this signature.
//   - Successes is the running count of recipe applications that
//     ultimately recovered the gate.
//   - LastHitAt is the wall-clock time of the most recent lookup hit.
type Recipe struct {
	ID                uuid.UUID
	FailureSignature  string
	Category          string
	Fix               map[string]any
	Hits              int
	Successes         int
	LastHitAt         time.Time
	CreatedAt         time.Time
}

// Genome is the read/write contract for the repair recipe registry.
//
//   - Record upserts the (signature, category, fix) tuple. If the
//     signature already exists we keep the existing recipe and only
//     bump bookkeeping fields.
//   - Lookup returns the recipe for a signature and increments Hits +
//     LastHitAt on a match. The boolean is false when no recipe
//     matches.
//   - MarkSuccess increments the Successes counter for the signature.
//   - Top returns the most-used recipes (by Hits), capped to limit.
//   - AttemptsByExecution returns the per-execution recovery attempts
//     the genome was consulted for. The repair genome itself is keyed
//     by failure signature (not execution) so this method returns an
//     empty slice today — the wow-loop adapter prefers the
//     execution.Service.RecoveryAttemptsByExecution path, which reads
//     recovery.recipe_*.v1 events out of execution_events. See the
//     method comment for details.
type Genome interface {
	Record(ctx context.Context, failureSignature, category string, fix map[string]any) (Recipe, error)
	Lookup(ctx context.Context, signature string) (Recipe, bool, error)
	MarkSuccess(ctx context.Context, signature string) error
	Top(ctx context.Context, limit int) ([]Recipe, error)
	AttemptsByExecution(ctx context.Context, executionID string) ([]Attempt, error)
}

// Attempt is one recovery attempt as projected for the wow-loop
// RepairSource adapter. Mirrors the shape execution.RecoveryAttempt
// returns from execution_events; we keep a parallel type here so the
// repair package can serve the wow-loop directly when (a future
// version of) the genome starts indexing attempts by executionID.
type Attempt struct {
	FailureSignature string
	Gate             string
	Applied          bool
	Success          bool
	OccurredAt       time.Time
}
