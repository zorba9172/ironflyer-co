package operator

import (
	"context"
	"time"

	"ironflyer/apps/orchestrator/internal/abuse"
	"ironflyer/apps/orchestrator/internal/audit"
	"ironflyer/apps/orchestrator/internal/deploy"
	"ironflyer/apps/orchestrator/internal/execution"
	"ironflyer/apps/orchestrator/internal/wallet"
)

// Deps is the OperatorService constructor envelope. Every field is
// nil-safe at call time: a method that needs a missing dependency
// returns ErrNotConfigured so the operator surface degrades gracefully
// when main.go has not wired the matching V22 service.
type Deps struct {
	// Deploy backs PendingApprovals.
	Deploy deploy.Service
	// Abuse backs AbuseScore.
	Abuse abuse.Engine
	// Execution + SandboxCapacity back ScaleSnapshot.
	Execution       execution.Service
	SandboxCapacity int
	// Wallet backs WalletSnapshot.
	Wallet wallet.Service
	// Audit backs AuditCursor.
	Audit audit.Store

	// MaxAuditLimit caps AuditCursor.limit (default 1000 when zero).
	MaxAuditLimit int
}

// service is the canonical OperatorService implementation: it wraps
// the V22 services and re-shapes their results into operator-facing
// payloads.
type service struct {
	d Deps
}

// New builds an OperatorService from the supplied dependency set.
// The constructor does not validate the deps — every method handles
// nil-safety itself so a partial deploy that wires only some surfaces
// still boots.
func New(d Deps) OperatorService {
	if d.MaxAuditLimit <= 0 {
		d.MaxAuditLimit = 1000
	}
	return &service{d: d}
}

func (s *service) PendingApprovals(ctx context.Context, tenantID string) ([]deploy.Approval, error) {
	if err := RequireOperator(ctx); err != nil {
		return nil, err
	}
	if s.d.Deploy == nil {
		return nil, ErrNotConfigured
	}
	if tenantID != "" {
		return s.d.Deploy.PendingApprovals(ctx, tenantID)
	}
	// No tenant filter — enumerate every tenant with pending work and
	// flatten so the operator sees the full backlog in one call.
	tenants, err := s.d.Deploy.TenantsWithPendingApprovals(ctx)
	if err != nil {
		return nil, err
	}
	var out []deploy.Approval
	for _, t := range tenants {
		rows, err := s.d.Deploy.PendingApprovals(ctx, t)
		if err != nil {
			return nil, err
		}
		out = append(out, rows...)
	}
	return out, nil
}

func (s *service) AbuseScore(ctx context.Context, tenantID, userID string) (int, string, error) {
	if err := RequireOperator(ctx); err != nil {
		return 0, "", err
	}
	if s.d.Abuse == nil {
		return 0, "", ErrNotConfigured
	}
	score, tier, err := s.d.Abuse.Score(ctx, tenantID, userID)
	if err != nil {
		return 0, "", err
	}
	return score, string(tier), nil
}

func (s *service) ScaleSnapshot(ctx context.Context) (ScaleSnapshot, error) {
	if err := RequireOperator(ctx); err != nil {
		return ScaleSnapshot{}, err
	}
	if s.d.Execution == nil {
		return ScaleSnapshot{}, ErrNotConfigured
	}
	active, err := s.d.Execution.ActiveCount(ctx)
	if err != nil {
		return ScaleSnapshot{}, err
	}
	queued, err := s.d.Execution.QueuedCount(ctx)
	if err != nil {
		return ScaleSnapshot{}, err
	}
	cap := s.d.SandboxCapacity
	util := 0.0
	if cap > 0 {
		util = float64(active) / float64(cap) * 100.0
	}
	return ScaleSnapshot{
		ActiveExecutions:     active,
		QueuedExecutions:     queued,
		SandboxCapacity:      cap,
		WorkerUtilizationPct: util,
	}, nil
}

func (s *service) WalletSnapshot(ctx context.Context, tenantID string) (WalletSnapshot, error) {
	if err := RequireOperator(ctx); err != nil {
		return WalletSnapshot{}, err
	}
	if s.d.Wallet == nil {
		return WalletSnapshot{}, ErrNotConfigured
	}
	w, err := s.d.Wallet.Get(ctx, tenantID)
	if err != nil {
		return WalletSnapshot{}, err
	}
	return WalletSnapshot{
		TenantID:         w.TenantID,
		BalanceUSD:       w.BalanceUSD,
		HoldUSD:          w.HoldUSD,
		LifetimeTopUpUSD: w.LifetimeTopUpUSD,
		LifetimeSpendUSD: w.LifetimeSpendUSD,
	}, nil
}

func (s *service) AuditCursor(ctx context.Context, since time.Time, limit int) ([]AuditEntry, error) {
	if err := RequireOperator(ctx); err != nil {
		return nil, err
	}
	if s.d.Audit == nil {
		return nil, ErrNotConfigured
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > s.d.MaxAuditLimit {
		limit = s.d.MaxAuditLimit
	}
	rows, err := s.d.Audit.Query(ctx, audit.Query{Since: since, Limit: limit})
	if err != nil {
		return nil, err
	}
	out := make([]AuditEntry, 0, len(rows))
	for _, e := range rows {
		out = append(out, AuditEntry{
			ID:        e.ID,
			Timestamp: e.CreatedAt,
			Action:    string(e.Action),
			Outcome:   string(e.Outcome),
			Hash:      e.ContentHash,
		})
	}
	return out, nil
}
