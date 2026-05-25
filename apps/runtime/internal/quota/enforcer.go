package quota

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
)

// AdmitRequest is the per-allocation admission payload. Resource
// fields are in integer units (1.0 CPU = 1) so the in-process
// counters stay cheap and lock-free-friendly.
type AdmitRequest struct {
	TenantID             string
	ExecutionID          string
	WorkspaceID          string
	RequestedCPU         int
	RequestedMemMB       int
	EstimatedDurationSec int
	RuntimeClass         string
	EstimatedCostUSD     decimal.Decimal
}

// Decision is what Admit returns. When Allow is false the caller MUST
// honour Reason and (optionally) Backoff before retrying.
type Decision struct {
	Allow   bool
	Reason  Reason
	Message string
	Retry   bool
	Backoff time.Duration
}

// Enforcer is the admission control surface. Implementations are safe
// for concurrent use.
type Enforcer interface {
	Admit(ctx context.Context, req AdmitRequest) (Decision, error)
	Release(ctx context.Context, tenantID, executionID, workspaceID string) error
	UsageSnapshot(ctx context.Context, tenantID string) (Usage, error)
}

// StandardEnforcer is the default Enforcer wired against a Store and a
// quota Config.
type StandardEnforcer struct {
	cfg     Config
	store   Store
	logger  zerolog.Logger
	metrics *Metrics
}

// NewStandardEnforcer builds the default admission enforcer.
func NewStandardEnforcer(cfg Config, store Store, logger zerolog.Logger) *StandardEnforcer {
	if store == nil {
		store = NewMemoryStore()
	}
	return &StandardEnforcer{
		cfg:     cfg,
		store:   store,
		logger:  logger.With().Str("component", "quota").Logger(),
		metrics: &Metrics{},
	}
}

// MetricsView exposes the in-process counter set.
func (e *StandardEnforcer) MetricsView() *Metrics { return e.metrics }

// Admit implements Enforcer. v1 enforces the per-tenant TenantQuota
// holistically; execution/workspace/node bounds are validated as a
// best-effort additional guard but the canonical hold is per-tenant.
func (e *StandardEnforcer) Admit(ctx context.Context, req AdmitRequest) (Decision, error) {
	if req.TenantID == "" {
		// Without a tenant we have nothing to enforce against; this
		// is allowed only because dev / mock-driver paths may run
		// anonymously. The orchestrator sets a tenant in prod.
		e.metrics.ObserveAdmit(true, "")
		return Decision{Allow: true}, nil
	}
	if req.EstimatedDurationSec > 0 &&
		e.cfg.DefaultExecution.MaxWallSeconds > 0 &&
		req.EstimatedDurationSec > e.cfg.DefaultExecution.MaxWallSeconds {
		e.metrics.ObserveAdmit(false, ReasonQuotaExceeded)
		return Decision{Allow: false, Reason: ReasonQuotaExceeded, Message: "execution wall budget exceeded"}, nil
	}
	if !e.cfg.DefaultExecution.MaxEstimatedCostUSD.IsZero() &&
		req.EstimatedCostUSD.GreaterThan(e.cfg.DefaultExecution.MaxEstimatedCostUSD) {
		e.metrics.ObserveAdmit(false, ReasonStopLoss)
		return Decision{Allow: false, Reason: ReasonStopLoss, Message: "execution cost ceiling"}, nil
	}
	lease := Lease{
		TenantID:     req.TenantID,
		ExecutionID:  req.ExecutionID,
		WorkspaceID:  req.WorkspaceID,
		CPU:          req.RequestedCPU,
		MemMB:        req.RequestedMemMB,
		RuntimeClass: req.RuntimeClass,
		EstUSD:       req.EstimatedCostUSD,
	}
	if err := e.store.Hold(ctx, req.TenantID, e.cfg.DefaultTenant, lease); err != nil {
		if qe, ok := err.(*Error); ok {
			e.metrics.ObserveAdmit(false, qe.Reason)
			retry := qe.Reason == ReasonCapacityWait || qe.Reason == ReasonPauseForBudget
			return Decision{
				Allow:   false,
				Reason:  qe.Reason,
				Message: qe.Message,
				Retry:   retry,
				Backoff: defaultBackoff(qe.Reason),
			}, nil
		}
		return Decision{}, err
	}
	e.metrics.ObserveAdmit(true, "")
	return Decision{Allow: true}, nil
}

// Release implements Enforcer.
func (e *StandardEnforcer) Release(ctx context.Context, tenantID, executionID, workspaceID string) error {
	e.metrics.ObserveRelease()
	return e.store.Release(ctx, tenantID, executionID, workspaceID)
}

// UsageSnapshot implements Enforcer.
func (e *StandardEnforcer) UsageSnapshot(ctx context.Context, tenantID string) (Usage, error) {
	return e.store.Get(ctx, tenantID)
}

// defaultBackoff is the suggested retry delay per reason.
func defaultBackoff(r Reason) time.Duration {
	switch r {
	case ReasonCapacityWait:
		return 5 * time.Second
	case ReasonPauseForBudget:
		return 30 * time.Second
	default:
		return 0
	}
}

var _ Enforcer = (*StandardEnforcer)(nil)
