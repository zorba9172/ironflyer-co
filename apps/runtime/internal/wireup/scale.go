// Package wireup is the runtime integration glue. It bridges the V22
// Wave-2 scale packages (snapshots, quota, warmpool, allocator,
// runtimeclass) into one cohesive admission funnel mounted ahead of
// the workspace manager in cmd/runtime/main.go.
package wireup

import (
	"context"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"ironflyer/apps/runtime/internal/allocator"
	"ironflyer/apps/runtime/internal/quota"
	"ironflyer/apps/runtime/internal/runtimeclass"
	"ironflyer/apps/runtime/internal/sandbox"
	"ironflyer/apps/runtime/internal/snapshots"
	"ironflyer/apps/runtime/internal/warmpool"
)

// ScaleResult bundles the constructed scale plane. The allocator is
// the headline admission seam; the snapshot manager backs the docker
// driver's SnapshotShim; the drainer goroutine keeps the warm pool
// from growing without bound while idle.
type ScaleResult struct {
	Allocator    allocator.Allocator
	Snapshots    *snapshots.S3Manager
	Quota        quota.Enforcer
	WarmPool     warmpool.Pool
	RuntimeClass *runtimeclass.StandardSelector
	Drainer      *warmpool.Drainer
}

// BuildScale stands up the runtime scale plane. snapshotCfg is
// supplied separately so the caller controls the SSE/KMS toggle from
// env. cfgPath is the optional runtime-class file (unused in v1).
func BuildScale(ctx context.Context, snapshotCfg snapshots.Config, log zerolog.Logger) (ScaleResult, error) {
	snapMgr, err := snapshots.NewS3Manager(ctx, snapshotCfg, log.With().Str("svc", "snapshots").Logger())
	if err != nil {
		return ScaleResult{}, err
	}

	// Quota — defaults boot empty; operators populate the YAML knob set
	// later. The enforcer treats zero as "no limit" so dev still admits.
	enforcer := quota.NewStandardEnforcer(quota.Config{}, quota.NewMemoryStore(), log)

	// Warm pool with defaults; the drainer goroutine handles cooldown.
	wpCfg := warmpool.DefaultConfig()
	pool := warmpool.New(wpCfg, log.With().Str("svc", "warmpool").Logger())
	drainer := warmpool.NewDrainer(pool, 30*time.Second, time.Minute, log)

	policy := runtimeclass.NewPolicy()
	selector := runtimeclass.NewSelector(policy)

	alloc := allocator.New(allocator.Config{ColdStartSLA: 0, AllowAnonymousTenant: true},
		enforcer, pool, selector, log)
	return ScaleResult{
		Allocator:    alloc,
		Snapshots:    snapMgr,
		Quota:        enforcer,
		WarmPool:     pool,
		RuntimeClass: selector,
		Drainer:      drainer,
	}, nil
}

// SnapshotShimAdapter satisfies sandbox.SnapshotShim by translating the
// (snapshotURI, dir) tuple the docker driver uses into the
// snapshots.S3Manager's workspaceID-keyed Restore / Checkpoint API.
// Snapshot URIs are interpreted as `s3://bucket/workspaces/<id>/...`;
// the adapter extracts the workspace id segment so the manager can
// look up its layout.
type SnapshotShimAdapter struct {
	Mgr *snapshots.S3Manager
}

// Restore implements sandbox.SnapshotShim. The URI's last segment
// after `workspaces/` is treated as the workspace id.
func (a SnapshotShimAdapter) Restore(ctx context.Context, snapshotURI, destDir string) error {
	if a.Mgr == nil || !a.Mgr.Enabled() {
		return nil
	}
	wsID := workspaceFromURI(snapshotURI)
	_, err := a.Mgr.RestoreLatest(ctx, wsID, destDir)
	return err
}

// Checkpoint implements sandbox.SnapshotShim.
func (a SnapshotShimAdapter) Checkpoint(ctx context.Context, srcDir, destSnapshotURI string) error {
	if a.Mgr == nil || !a.Mgr.Enabled() {
		return nil
	}
	wsID := workspaceFromURI(destSnapshotURI)
	_, err := a.Mgr.Checkpoint(ctx, wsID, srcDir, snapshots.CheckpointAfterPatch)
	return err
}

// workspaceFromURI returns the segment immediately after `workspaces/`
// in the URI. Falls back to the raw URI when no `workspaces/` segment
// is found so callers in dev paths still get a stable key.
func workspaceFromURI(uri string) string {
	const marker = "workspaces/"
	idx := strings.Index(uri, marker)
	if idx < 0 {
		return uri
	}
	rest := uri[idx+len(marker):]
	if slash := strings.IndexByte(rest, '/'); slash >= 0 {
		return rest[:slash]
	}
	return rest
}

// Compile-time assertion: keep the SnapshotShim contract synced.
var _ sandbox.SnapshotShim = SnapshotShimAdapter{}
