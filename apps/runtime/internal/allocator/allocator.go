package allocator

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"

	"ironflyer/apps/runtime/internal/quota"
	"ironflyer/apps/runtime/internal/runtimeclass"
	"ironflyer/apps/runtime/internal/warmpool"
)

// AllocateRequest is the per-create payload the allocator consumes.
type AllocateRequest struct {
	TenantID             string
	ExecutionID          string
	WorkspaceID          string
	RequestedCPU         int
	RequestedMemMB       int
	EstimatedCostUSD     decimal.Decimal
	RuntimeClass         string // optional override; selector decides if empty
	EstimatedDurationSec int
	SLAMaxColdStartSec   int
}

// Allocation is the allocator's verdict. When Allow is false the
// caller MUST honour Reason; Source/RuntimeClass are populated only
// on Allow.
type Allocation struct {
	Allow        bool
	WorkspaceID  string
	Source       string // "warm_pool" | "cold_start"
	LeaseID      string // populated when Source == "warm_pool"
	RuntimeClass string
	Reason       quota.Reason
	Message      string
	Retry        bool
	Backoff      time.Duration
}

// Allocator decides whether to bring a sandbox up, and where from.
type Allocator interface {
	Allocate(ctx context.Context, req AllocateRequest) (Allocation, error)
	// Release returns a warm-pool lease and drops the tenant's
	// quota hold for the workspace.
	Release(ctx context.Context, tenantID, executionID, workspaceID, leaseID string) error
}

// StandardAllocator wires the quota enforcer, the warm pool, and the
// runtime-class selector into one admission funnel.
type StandardAllocator struct {
	cfg      Config
	enforcer quota.Enforcer
	pool     warmpool.Pool
	selector runtimeclass.Selector
	logger   zerolog.Logger
}

// New builds the standard allocator.
func New(
	cfg Config,
	enforcer quota.Enforcer,
	pool warmpool.Pool,
	selector runtimeclass.Selector,
	logger zerolog.Logger,
) *StandardAllocator {
	if selector == nil {
		selector = runtimeclass.NewSelector(runtimeclass.NewPolicy())
	}
	return &StandardAllocator{
		cfg:      cfg,
		enforcer: enforcer,
		pool:     pool,
		selector: selector,
		logger:   logger.With().Str("component", "allocator").Logger(),
	}
}

// Allocate implements Allocator. Steps execute in the order the spec
// mandates; any failure short-circuits with a typed reason.
func (a *StandardAllocator) Allocate(ctx context.Context, req AllocateRequest) (Allocation, error) {
	// Step 1: wallet hold (caller-supplied via ctx).
	if !walletHoldOK(ctx) {
		return Allocation{
			Allow:   false,
			Reason:  quota.ReasonPauseForBudget,
			Message: "missing wallet hold",
		}, nil
	}
	// Step 2: ProfitGuard verdict (caller-supplied via ctx).
	if !profitGuardPositive(ctx) {
		return Allocation{
			Allow:   false,
			Reason:  quota.ReasonStopLoss,
			Message: "profitguard verdict not positive",
		}, nil
	}
	// Resolve RuntimeClass before quota — the rate depends on class.
	chosenClass := req.RuntimeClass
	if chosenClass == "" {
		chosenClass = a.selector.Select(ctx, req.TenantID, riskOf(ctx))
	}
	if !runtimeclass.IsKnown(chosenClass) {
		chosenClass = runtimeclass.ClassDocker
	}
	// Step 3: tenant quota.
	decision, err := a.enforcer.Admit(ctx, quota.AdmitRequest{
		TenantID:             req.TenantID,
		ExecutionID:          req.ExecutionID,
		WorkspaceID:          req.WorkspaceID,
		RequestedCPU:         req.RequestedCPU,
		RequestedMemMB:       req.RequestedMemMB,
		EstimatedDurationSec: req.EstimatedDurationSec,
		RuntimeClass:         chosenClass,
		EstimatedCostUSD:     req.EstimatedCostUSD,
	})
	if err != nil {
		return Allocation{}, err
	}
	if !decision.Allow {
		return Allocation{
			Allow:   false,
			Reason:  decision.Reason,
			Message: decision.Message,
			Retry:   decision.Retry,
			Backoff: decision.Backoff,
		}, nil
	}
	// Step 4: warm slot or cold start.
	source := "cold_start"
	var leaseID warmpool.LeaseID
	if a.shouldTryWarm(req) {
		id, lerr := a.pool.Lease(ctx, chosenClass)
		switch {
		case lerr == nil:
			leaseID = id
			source = "warm_pool"
		case errors.Is(lerr, warmpool.ErrNoSlot):
			// Fall through to cold start.
		default:
			// Pool errors are not fatal — fall through to cold start
			// but log so the warmer can be inspected.
			a.logger.Warn().Err(lerr).Str("class", chosenClass).Msg("warm lease error; cold-start fallback")
		}
	}
	// Step 5: node pool capacity — v1 logs only.
	a.logger.Debug().
		Str("workspace", req.WorkspaceID).
		Str("tenant", req.TenantID).
		Str("class", chosenClass).
		Str("source", source).
		Msg("allocator: node pool check deferred to kubernetes scheduler")

	return Allocation{
		Allow:        true,
		WorkspaceID:  req.WorkspaceID,
		Source:       source,
		LeaseID:      string(leaseID),
		RuntimeClass: chosenClass,
	}, nil
}

// shouldTryWarm decides whether to attempt a warm lease first. When
// the request explicitly tolerates a cold start longer than its SLA
// we skip the warm pool to leave warm slots for tighter SLAs.
func (a *StandardAllocator) shouldTryWarm(req AllocateRequest) bool {
	if a.pool == nil {
		return false
	}
	if req.SLAMaxColdStartSec <= 0 {
		return true
	}
	if a.cfg.ColdStartSLA <= 0 {
		return true
	}
	return time.Duration(req.SLAMaxColdStartSec)*time.Second <= a.cfg.ColdStartSLA
}

// Release implements Allocator.
func (a *StandardAllocator) Release(ctx context.Context, tenantID, executionID, workspaceID, leaseID string) error {
	if leaseID != "" && a.pool != nil {
		_ = a.pool.Return(ctx, warmpool.LeaseID(leaseID))
	}
	if a.enforcer != nil {
		return a.enforcer.Release(ctx, tenantID, executionID, workspaceID)
	}
	return nil
}

var _ Allocator = (*StandardAllocator)(nil)
