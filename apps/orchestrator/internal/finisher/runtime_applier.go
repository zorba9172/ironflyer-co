package finisher

import (
	"context"

	"ironflyer/apps/orchestrator/internal/patch"
)

// RuntimeApplier writes an approved patch into the user's live workspace.
// The finisher loop produces validated patches and updates the in-memory
// project, but materialising those bytes onto disk inside the workspace
// sandbox is owned by the runtime package. This interface is the seam.
//
// Implementations MUST:
//   - be idempotent on retry (the loop may re-apply on transient errors)
//   - respect the workspace boundary (never write outside the project tree)
//   - return a context.Canceled-shaped error if the caller's ctx is done
//
// A nil RuntimeApplier on Engine is a valid configuration — the loop will
// mark the patch applied in-memory only, which is the right behaviour
// for offline / mock-driver setups and for the unit test harness.
type RuntimeApplier interface {
	Apply(ctx context.Context, userBearer, workspaceID string, p patch.Patch) error
}

// NoopRuntimeApplier is the default Engine.applier when none is wired in.
// It records nothing and returns nil. Tests and the in-memory dev mode rely
// on this so the loop still closes cleanly without a workspace runtime.
type NoopRuntimeApplier struct{}

// Apply is a no-op: returns nil unconditionally.
func (NoopRuntimeApplier) Apply(context.Context, string, string, patch.Patch) error {
	return nil
}
