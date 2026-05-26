package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"ironflyer/core/runtime/internal/operations/snapshot"
	"ironflyer/core/runtime/internal/operations/state"
)

// Portability bundles the dependencies that let any runtime pod take
// over any workspace. It's an additive layer on top of Lifecycle:
//
//   - State (state.Store) owns the metadata layer — ownership claims,
//     heartbeats, status — backed by Postgres in prod.
//   - Snapshots (snapshot.Manager) owns the content layer — gzip
//     tarballs of the workspace filesystem in S3.
//   - PodID identifies this pod within the StatefulSet (k8s injects
//     POD_NAME via the downward API). Used as the claim token.
//   - WorkingDir is the per-pod root that snapshots are restored into;
//     in prod this is an emptyDir mounted at /var/lib/ironflyer/live.
//   - StaleAfter is how long without a heartbeat before another pod is
//     allowed to reclaim the workspace. Defaults to 60s.
//   - HeartbeatEvery is the cadence the owning pod uses to refresh the
//     heartbeat. Defaults to 15s.
type Portability struct {
	State          state.Store
	Snapshots      *snapshot.Manager
	PodID          string
	WorkingDir     string
	StaleAfter     time.Duration
	HeartbeatEvery time.Duration
}

// SetPortability wires the dependencies. Safe to call at any time
// before the heartbeat loop is started.
func (a *API) SetPortability(p Portability) {
	if p.StaleAfter <= 0 {
		p.StaleAfter = 60 * time.Second
	}
	if p.HeartbeatEvery <= 0 {
		p.HeartbeatEvery = 15 * time.Second
	}
	a.portability = p
}

// portabilityRoutes wires the admin drain endpoint. The drain hook is
// the bridge between k8s pod lifecycle and the metadata store: when a
// pod begins terminating, k8s invokes preStop which POSTs to /admin/
// drain. The handler iterates every workspace this pod owns,
// checkpoints to S3, and releases the claim so another pod can pick
// up.
func (a *API) registerPortabilityRoutes(r chi.Router) {
	r.Post("/admin/drain", a.adminDrain)
}

// adminDrain is the preStop entry point. It is intentionally
// authentication-free (the route is only reachable from inside the pod
// network and gated by NetworkPolicy) so the kubelet's lifecycle hook
// can call it without juggling tokens.
func (a *API) adminDrain(w http.ResponseWriter, r *http.Request) {
	if a.portability.State == nil || a.portability.PodID == "" {
		writeJSON(w, http.StatusOK, map[string]any{"drained": 0, "reason": "portability disabled"})
		return
	}
	count := a.drainPod(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"drained": count})
}

// drainPod is the shared implementation called by both /admin/drain and
// the SIGTERM handler. Returns the number of workspaces successfully
// released.
func (a *API) drainPod(ctx context.Context) int {
	if a.portability.State == nil || a.portability.PodID == "" {
		return 0
	}
	owned, err := a.portability.State.OwnedBy(ctx, a.portability.PodID)
	if err != nil {
		a.logger.Warn().Err(err).Msg("drain: list owned workspaces")
		return 0
	}
	var ok int
	for _, rec := range owned {
		if err := a.checkpointAndRelease(ctx, rec); err != nil {
			a.logger.Warn().Err(err).Str("workspace", rec.ID).Msg("drain: release workspace")
			continue
		}
		ok++
	}
	a.logger.Info().Int("released", ok).Int("total", len(owned)).Msg("pod drained")
	return ok
}

// checkpointAndRelease is the per-workspace handoff: snapshot the
// filesystem to S3, then release the claim. Best-effort: we still
// release the claim even if the snapshot fails so the workspace isn't
// orphaned to a dead pod.
func (a *API) checkpointAndRelease(ctx context.Context, rec state.Record) error {
	if a.portability.Snapshots != nil && a.portability.Snapshots.Enabled() {
		localDir := a.workspaceLocalDir(rec.ID)
		if _, err := a.portability.Snapshots.Checkpoint(ctx, rec.ID, localDir); err != nil {
			a.logger.Warn().
				Err(err).
				Str("workspace", rec.ID).
				Msg("drain: checkpoint failed; releasing anyway so reaper can recover")
		}
	}
	return a.portability.State.Release(ctx, rec.ID, a.portability.PodID, state.StatusStopped)
}

// workspaceLocalDir returns the per-pod working directory for the
// workspace. In prod the pod has an emptyDir at WorkingDir (typically
// /var/lib/ironflyer/live) and every workspace gets a subdirectory
// named after its ID.
func (a *API) workspaceLocalDir(id string) string {
	root := strings.TrimRight(a.portability.WorkingDir, "/")
	if root == "" {
		root = "/var/lib/ironflyer/live"
	}
	return root + "/" + id
}

// StartHeartbeat launches the heartbeat goroutine. Returns immediately;
// the goroutine stops when ctx is cancelled. No-op when portability is
// disabled.
func (a *API) StartHeartbeat(ctx context.Context) {
	if a.portability.State == nil || a.portability.PodID == "" {
		return
	}
	go heartbeatLoop(ctx, a.logger, a.portability)
}

// StartReaper launches the reaper goroutine. Every pod runs one; the
// first to land the atomic UPDATE wins.
func (a *API) StartReaper(ctx context.Context) {
	if a.portability.State == nil {
		return
	}
	go reaperLoop(ctx, a.logger, a.portability)
}

// StartPortabilityWorkers is the package-level entry point used by
// cmd/runtime, which only has the http.Handler interface (not the
// *API). It validates the Portability config and spawns the heartbeat
// + reaper goroutines.
func StartPortabilityWorkers(ctx context.Context, p Portability, logger zerolog.Logger) {
	if p.State == nil {
		return
	}
	if p.StaleAfter <= 0 {
		p.StaleAfter = 60 * time.Second
	}
	if p.HeartbeatEvery <= 0 {
		p.HeartbeatEvery = 15 * time.Second
	}
	if p.PodID != "" {
		go heartbeatLoop(ctx, logger, p)
	}
	go reaperLoop(ctx, logger, p)
}

// heartbeatLoop refreshes every workspace this pod owns on cadence.
// One UPDATE per workspace is acceptable at the expected scale (a
// single runtime pod hosts O(dozens) of workspaces); when the count
// grows we can batch via UNNEST.
func heartbeatLoop(ctx context.Context, logger zerolog.Logger, p Portability) {
	tick := time.NewTicker(p.HeartbeatEvery)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			owned, err := p.State.OwnedBy(ctx, p.PodID)
			if err != nil {
				logger.Warn().Err(err).Msg("heartbeat: list owned")
				continue
			}
			for _, rec := range owned {
				if _, err := p.State.Heartbeat(ctx, rec.ID, p.PodID); err != nil {
					logger.Warn().Err(err).Str("workspace", rec.ID).Msg("heartbeat: write")
				}
			}
		}
	}
}

// reaperLoop scans for workspaces with stale heartbeats and atomically
// frees them. The next request that lands on any pod can then call
// Claim and take over.
func reaperLoop(ctx context.Context, logger zerolog.Logger, p Portability) {
	interval := p.HeartbeatEvery * 2
	if interval < 15*time.Second {
		interval = 15 * time.Second
	}
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			freed, err := p.State.Reap(ctx, p.StaleAfter)
			if err != nil {
				logger.Warn().Err(err).Msg("reaper: scan")
				continue
			}
			if len(freed) > 0 {
				logger.Info().Strs("workspaces", freed).Msg("reaper: freed stale workspaces")
			}
		}
	}
}
