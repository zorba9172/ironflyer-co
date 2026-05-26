package snapshots

import (
	"strconv"
	"strings"
)

// Layout owns the snapshot object-key scheme. Kept as a value type so
// alternate prefix policies (per-tenant, per-region) are a constructor
// argument rather than a global edit.
type Layout struct {
	// Prefix is the root in-bucket key (no leading/trailing slash).
	Prefix string
}

// NewLayout normalises prefix slashes. Empty prefix is allowed.
func NewLayout(prefix string) Layout {
	return Layout{Prefix: strings.Trim(prefix, "/")}
}

// WorkspaceDir returns the per-workspace directory prefix.
//
//	<prefix>/workspaces/<id>
func (l Layout) WorkspaceDir(workspaceID string) string {
	return l.join("workspaces", workspaceID)
}

// CheckpointKey returns the object key for one tarball.
//
//	<prefix>/workspaces/<id>/<ts>.tar.zst
func (l Layout) CheckpointKey(workspaceID string, unixSec int64) string {
	return l.join("workspaces", workspaceID, strconv.FormatInt(unixSec, 10)+".tar.zst")
}

// LatestKey returns the LATEST pointer key.
//
//	<prefix>/workspaces/<id>/LATEST
func (l Layout) LatestKey(workspaceID string) string {
	return l.join("workspaces", workspaceID, "LATEST")
}

// ArchiveKey returns the cold-storage archive key.
//
//	<prefix>/archives/<id>.tar.zst
func (l Layout) ArchiveKey(workspaceID string) string {
	return l.join("archives", workspaceID+".tar.zst")
}

func (l Layout) join(parts ...string) string {
	if l.Prefix == "" {
		return strings.Join(parts, "/")
	}
	return l.Prefix + "/" + strings.Join(parts, "/")
}
