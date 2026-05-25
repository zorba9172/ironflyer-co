package snapshots

import "time"

// CheckpointKind tags the lifecycle event that triggered a checkpoint.
// Wired into the snapshot metadata so cost attribution / audit can
// distinguish forced lifecycle saves from periodic ones.
type CheckpointKind string

const (
	// CheckpointAfterPatch fires once a finisher patch has been
	// applied successfully. Preserves user-visible progress.
	CheckpointAfterPatch CheckpointKind = "after_patch"
	// CheckpointAfterGate fires after a gate verdict of PASS. Locks
	// in a known-good state ProfitGuard can roll back to.
	CheckpointAfterGate CheckpointKind = "after_gate_pass"
	// CheckpointBeforeIdleTeardown fires from the idle scanner just
	// before compute is destroyed. Must succeed before tear-down.
	CheckpointBeforeIdleTeardown CheckpointKind = "before_idle_teardown"
	// CheckpointPeriodic fires from the periodic scheduler during
	// long-running executions. Bounded by the periodic interval.
	CheckpointPeriodic CheckpointKind = "periodic"
)

// Metadata is the row stored in Postgres for every snapshot object.
// Mirrors ARCHITECTURE_RUNTIME_SCALE.md "Workspace State And
// Snapshotting" / "Store metadata in Postgres".
type Metadata struct {
	WorkspaceID  string         `json:"workspaceId"`
	TenantID     string         `json:"tenantId,omitempty"`
	ExecutionID  string         `json:"executionId,omitempty"`
	ObjectKey    string         `json:"objectKey"`
	Bucket       string         `json:"bucket"`
	Checksum     string         `json:"checksum,omitempty"`     // sha256 hex
	SizeBytes    int64          `json:"sizeBytes"`
	Kind         CheckpointKind `json:"kind,omitempty"`
	CreatedAt    time.Time      `json:"createdAt"`
	RestoredAt   time.Time      `json:"restoredAt,omitempty"`
	LastTickAt   time.Time      `json:"lastTickAt,omitempty"`
	CompressedAs string         `json:"compressedAs,omitempty"` // "zstd"
}

// URI returns the s3://bucket/key location for logging / driver
// hand-off (e.g. Driver.RestoreFromSnapshot).
func (m Metadata) URI() string {
	if m.Bucket == "" || m.ObjectKey == "" {
		return ""
	}
	return "s3://" + m.Bucket + "/" + m.ObjectKey
}
