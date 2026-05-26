// Package snapshots owns the durable, object-store-first state for a
// workspace per ARCHITECTURE_RUNTIME_SCALE.md. It restores LATEST on
// cold start, checkpoints at lifecycle gates (after_patch, after_gate,
// before_idle_teardown, periodic), and archives idle workspaces to a
// cheaper key. S3 and R2 are both supported via the S3 API surface.
package snapshots

import "errors"

// ErrNotConfigured is returned when the manager is constructed without a
// bucket and a real operation (Restore/Checkpoint/Archive) is invoked.
// Callers that want a soft no-op should branch on Manager.Enabled().
var ErrNotConfigured = errors.New("snapshots: bucket not configured")

// ErrNoSnapshot signals a workspace has never been checkpointed.
// Restore returns this so callers can decide between "start empty"
// and "fail allocation".
var ErrNoSnapshot = errors.New("snapshots: no snapshot present")

// ErrPathEscape is returned by the tar extractor when an archive entry
// resolves outside the destination directory. Tar slip defence.
var ErrPathEscape = errors.New("snapshots: tar entry escapes destination")

// ErrUploadFailed wraps any S3 PutObject/multipart upload failure.
var ErrUploadFailed = errors.New("snapshots: upload failed")

// ErrDownloadFailed wraps any S3 GetObject/download failure.
var ErrDownloadFailed = errors.New("snapshots: download failed")
