package quota

import "github.com/shopspring/decimal"

// TenantQuota is the hard per-tenant ceiling enforced at admit time
// and rechecked between billing ticks. Zero on any field means "no
// limit" — the runtime never invents a quota the operator did not
// explicitly set.
type TenantQuota struct {
	// MaxConcurrentSandboxes caps the number of live sandboxes for
	// the tenant. The most commonly-tripped quota in steady state.
	MaxConcurrentSandboxes int
	// MaxConcurrentCPU is the sum of RequestedCPU across live
	// sandboxes (1.0 = one core).
	MaxConcurrentCPU int
	// MaxConcurrentMemMB is the sum of RequestedMemMB across live
	// sandboxes.
	MaxConcurrentMemMB int
	// MaxEgressGB is a rolling-window egress ceiling. v1 tracks it
	// at sandbox tick time; the enforcer counts increments only.
	MaxEgressGB int
	// MaxSnapshotGB caps total object-storage occupied by the
	// tenant's snapshots+archives.
	MaxSnapshotGB int
	// MaxSpendUSDPerDay is the soft cap above which new admissions
	// require degraded runtime or are paused.
	MaxSpendUSDPerDay decimal.Decimal
}

// ExecutionQuota bounds one execution.
type ExecutionQuota struct {
	MaxWallSeconds       int
	MaxSandboxTicks      int
	MaxRestoreBytes      int64
	MaxCheckpointBytes   int64
	MaxEstimatedCostUSD  decimal.Decimal
}

// WorkspaceQuota bounds one workspace.
type WorkspaceQuota struct {
	IdleTimeoutSeconds int
	MaxDiskMB          int
	MaxPreviewPorts    int
}

// NodeQuota bounds one runtime node. v1 enforces only the sandbox
// pod count; image-pull bandwidth and restore concurrency are tracked
// for observability and reserved for future enforcement.
type NodeQuota struct {
	MaxSandboxPods       int
	MaxImagePullMBPerSec int
	MaxRestoreConcurrent int
}

// RegionQuota bounds one region.
type RegionQuota struct {
	MaxPendingPaidQueue int
}

// Config bundles defaults for every scope. The integration agent loads
// these from env / config; the enforcer falls back on whichever
// fields are non-zero.
type Config struct {
	DefaultTenant    TenantQuota
	DefaultExecution ExecutionQuota
	DefaultWorkspace WorkspaceQuota
	DefaultNode      NodeQuota
	DefaultRegion    RegionQuota
}
