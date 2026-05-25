package repair

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// PatchEntry is one row in the patch_memory table.
//
//   - IntentSignature is the SHA-256 hex of (prompt + gates). It is a
//     non-unique key — one intent may have several recorded patch
//     shapes.
//   - Patch is the structured patch (free-form JSON). The finisher
//     interprets the shape.
//   - AffectedPaths is the list of paths the patch touched, so we can
//     filter / score by overlap with the current change set.
//   - CostUSD is the realised cost of producing the patch the first
//     time. Reused patches reuse the original cost as the "saved" value.
//   - AppliedCount is the number of times this patch was retrieved and
//     applied.
//   - SuccessCount is the number of times the application recovered
//     the intent (passed the gates).
type PatchEntry struct {
	ID              uuid.UUID
	IntentSignature string
	Patch           map[string]any
	AffectedPaths   []string
	CostUSD         decimal.Decimal
	AppliedCount    int
	SuccessCount    int
	CreatedAt       time.Time
	LastAppliedAt   time.Time
}

// Memory is the read/write contract for patch memory.
//
//   - Record inserts a new PatchEntry for the intent.
//   - Find returns every PatchEntry matching the intent signature.
//   - MarkApplied bumps AppliedCount and (optionally) SuccessCount on
//     the entry; updates LastAppliedAt.
type Memory interface {
	Record(ctx context.Context, intent string, patch map[string]any, paths []string, cost decimal.Decimal) (PatchEntry, error)
	Find(ctx context.Context, intent string) ([]PatchEntry, error)
	MarkApplied(ctx context.Context, id uuid.UUID, success bool) error
}
