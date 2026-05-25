package snapshots

import (
	"context"
	"time"
)

// Manager is the snapshot-plane API the rest of the runtime calls into.
// All implementations must be safe for concurrent use.
type Manager interface {
	// RestoreLatest reads LATEST for workspaceID, downloads that
	// snapshot, and extracts it into destDir. When no snapshot
	// exists the implementation returns (Metadata{}, ErrNoSnapshot)
	// so the caller can decide to start empty or fail allocation.
	RestoreLatest(ctx context.Context, workspaceID, destDir string) (Metadata, error)

	// Checkpoint builds a tar.zst of srcDir (minus configured
	// excludes), uploads it under the timestamped key, then flips
	// LATEST only after the put succeeds. kind is recorded on the
	// metadata row so audit knows why the snapshot exists.
	Checkpoint(ctx context.Context, workspaceID, srcDir string, kind CheckpointKind) (Metadata, error)

	// Archive copies the workspace's most recent checkpoint into
	// the long-lived archives/<id>.tar.zst slot. Returns the
	// archive's metadata. The caller is responsible for the
	// subsequent compute teardown.
	Archive(ctx context.Context, workspaceID string) (Metadata, error)

	// Delete removes all hot checkpoints for workspaceID after
	// confirming an archive object exists (or creating one if not).
	// Per spec: "Never delete object-store snapshots during request
	// handling" applies to archives; hot checkpoints are reapable.
	Delete(ctx context.Context, workspaceID string) error

	// Metadata returns the most recent Metadata row for workspaceID,
	// or ErrNoSnapshot if none exists.
	Metadata(ctx context.Context, workspaceID string) (Metadata, error)
}

// Now is the clock the package uses; swap from tests / replays via the
// allocator-level injection if needed.
var Now = func() time.Time { return time.Now().UTC() }
