package deploy

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
)

// MemoryService is the dev / no-Postgres Service. State lives in
// per-process maps; SubscribeEvents fan-out is in-process. Safe for
// concurrent use.
//
// The integration agent picks MemoryService when IRONFLYER_DB_DRIVER
// is "memory" (the dev default). Postgres deploys land in the
// sibling PostgresService.
type MemoryService struct {
	cfg     Config
	log     zerolog.Logger
	adapter map[Target]Adapter
	pg      ProfitGuardChecker

	mu        sync.RWMutex
	deploys   map[string]*Deploy
	approvals map[string]*Approval
	events    map[string][]Event

	subsMu sync.Mutex
	subs   map[string]map[chan Event]struct{}
}

// NewMemoryService constructs a MemoryService. adapters indexes the
// per-Target adapter (e.g. {TargetVercel: NewVercelAdapter(...),
// TargetNoop: NoopAdapter{}}). pg may be nil during dev — GuardDeploy
// treats nil as permissive.
func NewMemoryService(cfg Config, log zerolog.Logger, adapters map[Target]Adapter, pg ProfitGuardChecker) *MemoryService {
	if cfg.DefaultApprovalTTL <= 0 {
		cfg.DefaultApprovalTTL = 30 * time.Minute
	}
	if adapters == nil {
		adapters = map[Target]Adapter{}
	}
	return &MemoryService{
		cfg:       cfg,
		log:       log,
		adapter:   adapters,
		pg:        pg,
		deploys:   map[string]*Deploy{},
		approvals: map[string]*Approval{},
		events:    map[string][]Event{},
		subs:      map[string]map[chan Event]struct{}{},
	}
}

// Plan opens a new planned deploy.
func (s *MemoryService) Plan(ctx context.Context, in PlanInput) (Deploy, error) {
	if err := validatePlanInput(in); err != nil {
		return Deploy{}, err
	}
	adapter, ok := s.adapter[in.Target]
	if !ok {
		return Deploy{}, fmt.Errorf("%w: %s", ErrUnknownTarget, in.Target)
	}

	// Production deploys: ProfitGuard runs at Plan time so the row
	// never opens when the guard already wants to stop. Preview
	// deploys skip the guard.
	if in.Environment == EnvironmentProduction {
		if err := GuardDeploy(ctx, s.pg, in.Metadata, string(in.Environment)); err != nil {
			return Deploy{}, err
		}
	}

	planCtx := WithTenant(WithProject(ctx, in.ProjectID), in.TenantID)
	planRes, err := adapter.Plan(planCtx, in)
	if err != nil {
		return Deploy{}, fmt.Errorf("deploy: plan: %w", err)
	}

	now := time.Now().UTC()
	d := &Deploy{
		ID:           uuid.NewString(),
		TenantID:     in.TenantID,
		ProjectID:    in.ProjectID,
		ExecutionID:  in.ExecutionID,
		BlueprintID:  in.BlueprintID,
		Target:       in.Target,
		Environment:  in.Environment,
		Status:       StatusPlanned,
		DiffHash:     in.DiffHash,
		ArtifactHash: artifactHashFromMetadata(in),
		GateSummary:  copyMapString(in.GateSummary),
		CostUSD:      planRes.EstimatedCostUSD,
		Metadata:     copyMapAny(in.Metadata),
		CreatedAt:    now,
	}

	s.mu.Lock()
	s.deploys[d.ID] = d
	s.mu.Unlock()

	s.emit(d.ID, EventPlanned, map[string]any{
		"target":              string(in.Target),
		"environment":         string(in.Environment),
		"provider_project_id": planRes.ProviderProjectID,
	})
	return cloneDeploy(d), nil
}

// BuildPreview drives the preview build.
func (s *MemoryService) BuildPreview(ctx context.Context, deployID string) (Deploy, error) {
	d, adapter, err := s.lookupDeployAdapter(deployID)
	if err != nil {
		return Deploy{}, err
	}
	if d.Status != StatusPlanned && d.Status != StatusFailed {
		return Deploy{}, fmt.Errorf("%w: cannot build preview from %s", ErrInvalidState, d.Status)
	}

	s.updateStatus(d.ID, StatusPreviewBuilding, nil, nil, nil)
	s.emit(d.ID, EventPreviewBuilding, nil)

	previewCtx := WithTenant(WithProject(ctx, d.ProjectID), d.TenantID)
	plan := PlanResult{
		ProviderProjectID: deriveProviderProjectID(d),
		EstimatedCostUSD:  d.CostUSD,
	}
	res, err := adapter.BuildPreview(previewCtx, d.ID, plan)
	if err != nil {
		s.updateStatus(d.ID, StatusFailed, nil, nil, nil)
		s.emit(d.ID, EventFailed, map[string]any{"phase": "preview", "error": err.Error()})
		return Deploy{}, fmt.Errorf("deploy: build preview: %w", err)
	}

	now := time.Now().UTC()
	s.mu.Lock()
	row := s.deploys[d.ID]
	row.Status = StatusPreviewReady
	row.ProviderDeploymentID = res.ProviderDeploymentID
	row.PreviewURL = res.PreviewURL
	row.PreviewReadyAt = &now
	if res.CostUSD.IsPositive() {
		row.CostUSD = row.CostUSD.Add(res.CostUSD)
	}
	out := cloneDeploy(row)
	s.mu.Unlock()

	s.emit(d.ID, EventPreviewReady, map[string]any{
		"provider_deployment_id": res.ProviderDeploymentID,
		"preview_url":            res.PreviewURL,
		"cost_usd":               res.CostUSD.String(),
	})
	return out, nil
}

// RequestApproval opens a pending approval row.
func (s *MemoryService) RequestApproval(_ context.Context, deployID string, by UserRef, expiresIn time.Duration) (Approval, error) {
	s.mu.Lock()
	d, ok := s.deploys[deployID]
	if !ok {
		s.mu.Unlock()
		return Approval{}, ErrNotFound
	}
	if d.Status != StatusPreviewReady && d.Status != StatusAwaitingApproval {
		s.mu.Unlock()
		return Approval{}, fmt.Errorf("%w: cannot request approval from %s", ErrInvalidState, d.Status)
	}

	now := time.Now().UTC()
	a := &Approval{
		ID:                uuid.NewString(),
		DeployID:          d.ID,
		TenantID:          d.TenantID,
		RequestedByUserID: by.UserID,
		Status:            ApprovalPending,
		DiffHash:          d.DiffHash,
		ArtifactHash:      d.ArtifactHash,
		GateSummary:       copyMapString(d.GateSummary),
		CostImpactUSD:     d.CostUSD,
		ExpiresAt:         now.Add(defaultIfZero(expiresIn, s.cfg.DefaultApprovalTTL)),
		RequestedAt:       now,
	}
	s.approvals[a.ID] = a
	d.Status = StatusAwaitingApproval
	out := *a
	s.mu.Unlock()

	s.emit(deployID, EventApprovalRequest, map[string]any{
		"approval_id": a.ID,
		"expires_at":  a.ExpiresAt.Format(time.RFC3339),
	})
	return out, nil
}

// Decide flips an approval row.
func (s *MemoryService) Decide(_ context.Context, approvalID string, by UserRef, decision string, note string) (Approval, error) {
	verb, ok := normalizeDecision(decision)
	if !ok {
		return Approval{}, fmt.Errorf("%w: unknown decision %q", ErrInvalidState, decision)
	}
	now := time.Now().UTC()
	s.mu.Lock()
	a, ok := s.approvals[approvalID]
	if !ok {
		s.mu.Unlock()
		return Approval{}, ErrNotFound
	}
	if a.Status != ApprovalPending {
		s.mu.Unlock()
		return Approval{}, ErrApprovalNotPending
	}
	if !a.ExpiresAt.IsZero() && a.ExpiresAt.Before(now) {
		a.Status = ApprovalExpired
		a.DecidedAt = &now
		out := *a
		s.mu.Unlock()
		s.emit(a.DeployID, EventApprovalExpired, map[string]any{"approval_id": a.ID})
		return out, ErrApprovalExpired
	}

	switch verb {
	case DecisionApprove:
		a.Status = ApprovalApproved
	case DecisionReject:
		a.Status = ApprovalRejected
	}
	a.DecidedByUserID = by.UserID
	a.DecisionNote = note
	a.DecidedAt = &now

	if verb == DecisionReject {
		if d, ok := s.deploys[a.DeployID]; ok && d.Status == StatusAwaitingApproval {
			d.Status = StatusCancelled
		}
	}
	out := *a
	deployID := a.DeployID
	s.mu.Unlock()

	s.emit(deployID, EventApprovalDecided, map[string]any{
		"approval_id": out.ID,
		"decision":    verb,
		"note":        note,
	})
	if verb == DecisionReject {
		s.emit(deployID, EventCancelled, map[string]any{"reason": "approval_rejected"})
	}
	return out, nil
}

// Promote runs ProfitGuard + approval check, then asks the Adapter.
func (s *MemoryService) Promote(ctx context.Context, deployID string) (Deploy, error) {
	d, adapter, err := s.lookupDeployAdapter(deployID)
	if err != nil {
		return Deploy{}, err
	}
	if d.Status != StatusPreviewReady && d.Status != StatusAwaitingApproval {
		return Deploy{}, fmt.Errorf("%w: cannot promote from %s", ErrInvalidState, d.Status)
	}

	// Production deploys MUST have an approved approval row (or a
	// policy obligation, which the integration agent layers around
	// this call). Preview promotions are unusual but still require
	// approval — Promote is the "ship to production" verb.
	if d.Environment == EnvironmentProduction {
		if err := GuardDeploy(ctx, s.pg, d.Metadata, string(d.Environment)); err != nil {
			s.emit(d.ID, EventProfitGuardBlock, map[string]any{"error": err.Error()})
			return Deploy{}, err
		}
		rows := s.approvalsForDeploy(d.ID)
		latest := pickLatestApproval(rows)
		if !canPromote(latest, time.Now().UTC()) {
			return Deploy{}, ErrApprovalRequired
		}
	}

	s.updateStatus(d.ID, StatusPromoting, nil, nil, nil)
	s.emit(d.ID, EventPromoting, nil)

	promoteCtx := WithTenant(WithProject(ctx, d.ProjectID), d.TenantID)
	res, err := adapter.Promote(promoteCtx, d.ID, d.ProviderDeploymentID)
	if err != nil {
		s.updateStatus(d.ID, StatusFailed, nil, nil, nil)
		s.emit(d.ID, EventFailed, map[string]any{"phase": "promote", "error": err.Error()})
		return Deploy{}, fmt.Errorf("deploy: promote: %w", err)
	}

	now := time.Now().UTC()
	s.mu.Lock()
	row := s.deploys[d.ID]
	row.Status = StatusPromoted
	row.ProductionURL = res.ProductionURL
	row.PromotedAt = &now
	if res.CostUSD.IsPositive() {
		row.CostUSD = row.CostUSD.Add(res.CostUSD)
	}
	out := cloneDeploy(row)
	s.mu.Unlock()

	s.emit(d.ID, EventPromoted, map[string]any{
		"production_url": res.ProductionURL,
		"cost_usd":       res.CostUSD.String(),
	})
	return out, nil
}

// Rollback drives a production rollback.
func (s *MemoryService) Rollback(ctx context.Context, deployID, reason string) (Deploy, error) {
	d, adapter, err := s.lookupDeployAdapter(deployID)
	if err != nil {
		return Deploy{}, err
	}
	if d.Status != StatusPromoted {
		return Deploy{}, fmt.Errorf("%w: cannot rollback from %s", ErrInvalidState, d.Status)
	}
	rollCtx := WithTenant(WithProject(ctx, d.ProjectID), d.TenantID)
	res, err := adapter.Rollback(rollCtx, d.ID, d.ProviderDeploymentID, "")
	if err != nil {
		s.emit(d.ID, EventFailed, map[string]any{"phase": "rollback", "error": err.Error()})
		return Deploy{}, fmt.Errorf("deploy: rollback: %w", err)
	}
	now := time.Now().UTC()
	s.mu.Lock()
	row := s.deploys[d.ID]
	row.Status = StatusRolledBack
	row.RolledBackAt = &now
	out := cloneDeploy(row)
	s.mu.Unlock()
	s.emit(d.ID, EventRolledBack, map[string]any{
		"to_version": res.ToVersion,
		"reason":     reason,
	})
	return out, nil
}

// Cancel terminates a non-promoted deploy.
func (s *MemoryService) Cancel(_ context.Context, deployID, reason string) (Deploy, error) {
	s.mu.Lock()
	d, ok := s.deploys[deployID]
	if !ok {
		s.mu.Unlock()
		return Deploy{}, ErrNotFound
	}
	switch d.Status {
	case StatusPromoted:
		s.mu.Unlock()
		return Deploy{}, fmt.Errorf("%w: cannot cancel promoted deploy (use rollback)", ErrInvalidState)
	case StatusRolledBack, StatusCancelled, StatusFailed:
		out := cloneDeploy(d)
		s.mu.Unlock()
		return out, nil
	}
	d.Status = StatusCancelled
	out := cloneDeploy(d)
	s.mu.Unlock()
	s.emit(deployID, EventCancelled, map[string]any{"reason": reason})
	return out, nil
}

// GetByExecution returns the most recent deploy whose ExecutionID
// matches. Returns (zero, false, nil) when no deploy exists for the
// given execution.
func (s *MemoryService) GetByExecution(_ context.Context, executionID string) (Deploy, bool, error) {
	if executionID == "" {
		return Deploy{}, false, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var (
		latest    *Deploy
		latestAt  time.Time
	)
	for _, d := range s.deploys {
		if d.ExecutionID != executionID {
			continue
		}
		if latest == nil || d.CreatedAt.After(latestAt) {
			latest = d
			latestAt = d.CreatedAt
		}
	}
	if latest == nil {
		return Deploy{}, false, nil
	}
	return cloneDeploy(latest), true, nil
}

// Get returns a deploy or ErrNotFound.
func (s *MemoryService) Get(_ context.Context, id string) (Deploy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.deploys[id]
	if !ok {
		return Deploy{}, ErrNotFound
	}
	return cloneDeploy(d), nil
}

// List returns the most-recent deploys for a tenant.
func (s *MemoryService) List(_ context.Context, tenant string, limit, offset int) ([]Deploy, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	s.mu.RLock()
	rows := make([]Deploy, 0, len(s.deploys))
	for _, d := range s.deploys {
		if d.TenantID != tenant {
			continue
		}
		rows = append(rows, cloneDeploy(d))
	}
	s.mu.RUnlock()
	sort.Slice(rows, func(i, j int) bool { return rows[i].CreatedAt.After(rows[j].CreatedAt) })
	if offset >= len(rows) {
		return nil, nil
	}
	end := offset + limit
	if end > len(rows) {
		end = len(rows)
	}
	return rows[offset:end], nil
}

// PendingApprovals returns pending approvals for a tenant.
func (s *MemoryService) PendingApprovals(_ context.Context, tenant string) ([]Approval, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Approval, 0)
	for _, a := range s.approvals {
		if a.TenantID != tenant {
			continue
		}
		if a.Status != ApprovalPending {
			continue
		}
		out = append(out, *a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RequestedAt.After(out[j].RequestedAt) })
	return out, nil
}

// TenantsWithPendingApprovals returns distinct tenant ids that hold at
// least one pending approval row. Used by the expiry sweeper.
func (s *MemoryService) TenantsWithPendingApprovals(_ context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	seen := map[string]struct{}{}
	for _, a := range s.approvals {
		if a.Status != ApprovalPending {
			continue
		}
		if a.TenantID == "" {
			continue
		}
		seen[a.TenantID] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	sort.Strings(out)
	return out, nil
}

// RecordCost bumps the cost counter.
func (s *MemoryService) RecordCost(_ context.Context, deployID string, addedUSD decimal.Decimal) error {
	if !addedUSD.IsPositive() {
		return nil
	}
	s.mu.Lock()
	d, ok := s.deploys[deployID]
	if !ok {
		s.mu.Unlock()
		return ErrNotFound
	}
	d.CostUSD = d.CostUSD.Add(addedUSD)
	s.mu.Unlock()
	s.emit(deployID, EventCostRecorded, map[string]any{"added_usd": addedUSD.String()})
	return nil
}

// SubscribeEvents fan-outs in-process events for one deploy.
func (s *MemoryService) SubscribeEvents(ctx context.Context, deployID string) (<-chan Event, error) {
	s.mu.RLock()
	_, ok := s.deploys[deployID]
	s.mu.RUnlock()
	if !ok {
		return nil, ErrNotFound
	}
	ch := make(chan Event, 16)
	s.subsMu.Lock()
	subs, ok := s.subs[deployID]
	if !ok {
		subs = map[chan Event]struct{}{}
		s.subs[deployID] = subs
	}
	subs[ch] = struct{}{}
	s.subsMu.Unlock()

	// Replay history so the subscriber sees what already happened.
	s.mu.RLock()
	history := append([]Event(nil), s.events[deployID]...)
	s.mu.RUnlock()
	go func() {
		defer func() {
			s.subsMu.Lock()
			if subs, ok := s.subs[deployID]; ok {
				delete(subs, ch)
				if len(subs) == 0 {
					delete(s.subs, deployID)
				}
			}
			s.subsMu.Unlock()
			close(ch)
		}()
		for _, ev := range history {
			select {
			case <-ctx.Done():
				return
			case ch <- ev:
			}
		}
		<-ctx.Done()
	}()
	return ch, nil
}

// ---- internals ----------------------------------------------------

func (s *MemoryService) lookupDeployAdapter(id string) (*Deploy, Adapter, error) {
	s.mu.RLock()
	d, ok := s.deploys[id]
	s.mu.RUnlock()
	if !ok {
		return nil, nil, ErrNotFound
	}
	adapter, ok := s.adapter[d.Target]
	if !ok {
		return nil, nil, fmt.Errorf("%w: %s", ErrUnknownTarget, d.Target)
	}
	return d, adapter, nil
}

func (s *MemoryService) updateStatus(id string, status Status, previewAt, promotedAt, rolledAt *time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.deploys[id]
	if !ok {
		return
	}
	d.Status = status
	if previewAt != nil {
		d.PreviewReadyAt = previewAt
	}
	if promotedAt != nil {
		d.PromotedAt = promotedAt
	}
	if rolledAt != nil {
		d.RolledBackAt = rolledAt
	}
}

func (s *MemoryService) approvalsForDeploy(deployID string) []Approval {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Approval, 0)
	for _, a := range s.approvals {
		if a.DeployID == deployID {
			out = append(out, *a)
		}
	}
	return out
}

func (s *MemoryService) emit(deployID, eventType string, payload map[string]any) {
	ev := Event{
		DeployID:  deployID,
		EventType: eventType,
		Payload:   payload,
		CreatedAt: time.Now().UTC(),
	}
	s.mu.Lock()
	s.events[deployID] = append(s.events[deployID], ev)
	s.mu.Unlock()

	s.subsMu.Lock()
	subs := s.subs[deployID]
	chans := make([]chan Event, 0, len(subs))
	for ch := range subs {
		chans = append(chans, ch)
	}
	s.subsMu.Unlock()
	for _, ch := range chans {
		select {
		case ch <- ev:
		default:
			// Slow subscriber; drop rather than block.
		}
	}
}

// validatePlanInput is the cheap input gate before we start
// allocating ids or talking to the adapter.
func validatePlanInput(in PlanInput) error {
	if in.TenantID == "" {
		return fmt.Errorf("%w: tenant_id required", ErrInvalidState)
	}
	if in.ProjectID == "" {
		return fmt.Errorf("%w: project_id required", ErrInvalidState)
	}
	if in.Target == "" {
		return fmt.Errorf("%w: target required", ErrInvalidState)
	}
	switch in.Environment {
	case EnvironmentPreview, EnvironmentProduction:
	default:
		return fmt.Errorf("%w: environment must be preview|production", ErrInvalidState)
	}
	return nil
}

func artifactHashFromMetadata(in PlanInput) string {
	if h := metaString(in.Metadata, "artifact_hash"); h != "" {
		return h
	}
	return ""
}

func deriveProviderProjectID(d *Deploy) string {
	if id := metaString(d.Metadata, "vercel_project_id"); id != "" {
		return id
	}
	if id := metaString(d.Metadata, "vercel_project_name"); id != "" {
		return id
	}
	return fmt.Sprintf("ironflyer-%s", shortHash(d.ProjectID))
}

func cloneDeploy(d *Deploy) Deploy {
	if d == nil {
		return Deploy{}
	}
	out := *d
	out.GateSummary = copyMapString(d.GateSummary)
	out.Metadata = copyMapAny(d.Metadata)
	return out
}

func copyMapString(in map[string]string) map[string]string {
	if in == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func copyMapAny(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
