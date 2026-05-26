// allocator.go wires Agent 23's allocator + quota enforcer into the
// HTTP create/destroy paths. Every workspace creation flows through
// allocator.Allocate (wallet hold → ProfitGuard → tenant quota → warm
// slot / cold start → runtime-class selection), and every teardown
// returns the warm-pool lease + drops the quota hold via
// allocator.Release. A GET /quota/usage endpoint exposes the live
// per-tenant snapshot for dashboards.
package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/shopspring/decimal"

	"ironflyer/core/runtime/internal/operations/allocator"
	"ironflyer/core/runtime/internal/operations/quota"
)

// allocRecord stores the inputs Release() needs after Create succeeds.
// Keyed by sandbox workspace ID so the destroy/archive handlers can
// look it up without dragging extra fields through sandbox.Workspace.
type allocRecord struct {
	TenantID    string
	ExecutionID string
	WorkspaceID string
	LeaseID     string
	Source      string
}

// allocTracker is the in-process map of live allocations. Production
// truth lives in the quota Store; this is only here so the destroy
// handler knows the original (tenantID, executionID, leaseID) tuple to
// hand back to allocator.Release. Concurrent access is gated by mu.
type allocTracker struct {
	mu   sync.RWMutex
	rows map[string]allocRecord
}

func newAllocTracker() *allocTracker {
	return &allocTracker{rows: make(map[string]allocRecord)}
}

func (t *allocTracker) put(rec allocRecord) {
	t.mu.Lock()
	t.rows[rec.WorkspaceID] = rec
	t.mu.Unlock()
}

func (t *allocTracker) get(id string) (allocRecord, bool) {
	t.mu.RLock()
	rec, ok := t.rows[id]
	t.mu.RUnlock()
	return rec, ok
}

func (t *allocTracker) drop(id string) {
	t.mu.Lock()
	delete(t.rows, id)
	t.mu.Unlock()
}

// allocateForCreate runs the 5-step admission funnel before the sandbox
// Manager.Create call. It mirrors the orchestrator's expectations: the
// caller (orchestrator) is the only authority on wallet hold +
// ProfitGuard, so those verdicts arrive as advisory HTTP headers and
// are stamped into context for the allocator to read.
//
// Returns the verdict + the (mutated) request body. The caller MUST
// stop on (allocation.Allow == false) and surface writeAllocatorError
// to map quota.Reason → HTTP status.
func (a *API) allocateForCreate(r *http.Request, tenantID, executionID, workspaceID, runtimeClass string, cpu, memMB, estDurationSec int, estCost decimal.Decimal) (context.Context, allocator.Allocation, error) {
	if a.allocator == nil {
		// Legacy / dev path: no allocator wired. Surface a "permit"
		// verdict so the create handler still flows through.
		return r.Context(), allocator.Allocation{
			Allow:        true,
			WorkspaceID:  workspaceID,
			Source:       "cold_start",
			RuntimeClass: runtimeClass,
		}, nil
	}

	ctx := r.Context()
	if strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Ironflyer-Wallet-Hold")), "ok") {
		ctx = allocator.WithWalletHold(ctx, true)
	}
	if strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Ironflyer-ProfitGuard")), "ok") {
		ctx = allocator.WithProfitGuard(ctx, true)
	}
	if risk := strings.TrimSpace(r.Header.Get("X-Ironflyer-Risk")); risk != "" {
		ctx = allocator.WithRisk(ctx, risk)
	}

	// Apply defaults so the allocator always has a meaningful payload
	// to evaluate (callers may legitimately omit any of these).
	if cpu <= 0 {
		cpu = 1
	}
	if memMB <= 0 {
		memMB = 1024
	}
	if estDurationSec <= 0 {
		estDurationSec = 600
	}

	req := allocator.AllocateRequest{
		TenantID:             tenantID,
		ExecutionID:          executionID,
		WorkspaceID:          workspaceID,
		RequestedCPU:         cpu,
		RequestedMemMB:       memMB,
		EstimatedCostUSD:     estCost,
		EstimatedDurationSec: estDurationSec,
		RuntimeClass:         runtimeClass,
		SLAMaxColdStartSec:   5,
	}
	alloc, err := a.allocator.Allocate(ctx, req)
	return ctx, alloc, err
}

// releaseAllocation hands back the warm-pool lease and drops the quota
// hold. It is idempotent — calling it twice for the same workspace is
// safe. The httpapi calls this from the destroy + archive handlers,
// and from the create handler's rollback path when Manager.Create
// fails after the allocator already admitted.
func (a *API) releaseAllocation(ctx context.Context, rec allocRecord) {
	if a.allocator == nil {
		return
	}
	if err := a.allocator.Release(ctx, rec.TenantID, rec.ExecutionID, rec.WorkspaceID, rec.LeaseID); err != nil {
		a.logger.Warn().Err(err).
			Str("workspace", rec.WorkspaceID).
			Str("tenant", rec.TenantID).
			Msg("allocator release failed")
	}
}

// writeAllocatorError maps an allocator failure into an HTTP response.
// The verdict's Reason is the contract the orchestrator already
// understands — surfacing the typed reason in the JSON body keeps the
// orchestrator's retry / top-up / degrade logic single-source.
func writeAllocatorError(w http.ResponseWriter, alloc allocator.Allocation, err error) {
	if err != nil {
		// Distinguish typed quota.Error (which Admit may also surface
		// directly) from anything unexpected the store may bubble up.
		var qErr *quota.Error
		if errors.As(err, &qErr) {
			writeJSON(w, statusForReason(qErr.Reason), map[string]any{
				"reason":  string(qErr.Reason),
				"message": qErr.Message,
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, errJSON(err.Error()))
		return
	}

	status := statusForReason(alloc.Reason)
	body := map[string]any{
		"reason":  string(alloc.Reason),
		"message": alloc.Message,
	}
	if alloc.Retry && alloc.Backoff > 0 {
		body["retry_after"] = int(alloc.Backoff.Seconds())
		w.Header().Set("Retry-After", strconv.Itoa(int(alloc.Backoff.Seconds())))
	}
	writeJSON(w, status, body)
}

// statusForReason maps the typed quota.Reason onto the HTTP status the
// orchestrator agrees on. 402 surfaces wallet/profit issues so the
// top-up UI fires; 429 maps to a hard quota lid; 503 maps to capacity
// pressure that a retry will resolve; 409 maps to "switch runtime
// class" which is a precondition mismatch.
func statusForReason(r quota.Reason) int {
	switch r {
	case quota.ReasonPauseForBudget:
		return http.StatusPaymentRequired
	case quota.ReasonStopLoss:
		return http.StatusPaymentRequired
	case quota.ReasonQuotaExceeded:
		return http.StatusTooManyRequests
	case quota.ReasonCapacityWait:
		return http.StatusServiceUnavailable
	case quota.ReasonDegradeRuntime:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

// estCostFromHeader parses the orchestrator-supplied expected cost
// (USD) header. Missing / unparseable values default to zero so the
// allocator skips the cost ceiling check.
func estCostFromHeader(r *http.Request) decimal.Decimal {
	v := strings.TrimSpace(r.Header.Get("X-Ironflyer-Estimated-Cost-USD"))
	if v == "" {
		return decimal.Zero
	}
	d, err := decimal.NewFromString(v)
	if err != nil {
		return decimal.Zero
	}
	return d
}

// quotaUsage is the GET /quota/usage handler. It returns the live
// counters the dashboards / orchestrator scale loop consume. The
// route is intentionally unauthenticated when the verifier is nil
// (dev / single-pod) — when JWT is enabled it sits behind the
// standard auth middleware below.
func (a *API) quotaUsage(w http.ResponseWriter, r *http.Request) {
	if a.quotaEnforcer == nil {
		writeJSON(w, http.StatusNotImplemented, errJSON("quota enforcer not configured"))
		return
	}
	tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
	if tenantID == "" {
		writeJSON(w, http.StatusBadRequest, errJSON("tenant_id required"))
		return
	}
	usage, err := a.quotaEnforcer.UsageSnapshot(r.Context(), tenantID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errJSON(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tenant_id":         usage.TenantID,
		"active_sandboxes":  usage.LiveSandboxes,
		"active_cpu_cores":  usage.LiveCPU,
		"active_memory_mb":  usage.LiveMemMB,
		"active_executions": usage.LiveExecutions,
		"daily_spend_usd":   usage.SpendTodayUSD.StringFixed(4),
	})
}
