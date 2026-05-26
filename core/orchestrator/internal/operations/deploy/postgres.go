package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
)

// PostgresService is the production-grade Service backed by the
// deploys / deploy_events / deploy_approvals tables added by
// migrations 00033 and 00034. The FSM and approval contract match
// MemoryService verbatim — both implementations pass the same test
// surface (when one ships) and behave identically at the resolver
// layer.
//
// SubscribeEvents currently fan-outs in-process events only — the
// Postgres rows are the durable trail, and the resolver replays
// existing rows from the table when a subscriber connects. The
// integration agent layers Redpanda fan-out on top once the outbox
// publisher is wired.
type PostgresService struct {
	cfg     Config
	log     zerolog.Logger
	pool    *pgxpool.Pool
	adapter map[Target]Adapter
	pg      ProfitGuardChecker

	subsMu sync.Mutex
	subs   map[string]map[chan Event]struct{}
}

// NewPostgresService wires the service to an existing pgxpool.
func NewPostgresService(pool *pgxpool.Pool, cfg Config, log zerolog.Logger, adapters map[Target]Adapter, pg ProfitGuardChecker) *PostgresService {
	if cfg.DefaultApprovalTTL <= 0 {
		cfg.DefaultApprovalTTL = 30 * time.Minute
	}
	if adapters == nil {
		adapters = map[Target]Adapter{}
	}
	return &PostgresService{
		cfg:     cfg,
		log:     log,
		pool:    pool,
		adapter: adapters,
		pg:      pg,
		subs:    map[string]map[chan Event]struct{}{},
	}
}

// Plan opens a planned deploy row.
func (s *PostgresService) Plan(ctx context.Context, in PlanInput) (Deploy, error) {
	if err := validatePlanInput(in); err != nil {
		return Deploy{}, err
	}
	adapter, ok := s.adapter[in.Target]
	if !ok {
		return Deploy{}, fmt.Errorf("%w: %s", ErrUnknownTarget, in.Target)
	}
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

	gateJSON, _ := json.Marshal(stringMapOrEmpty(in.GateSummary))
	metaJSON, _ := json.Marshal(anyMapOrEmpty(in.Metadata))
	execID := nullStringIfEmpty(in.ExecutionID)
	bpID := nullStringIfEmpty(in.BlueprintID)

	var id string
	var createdAt time.Time
	row := s.pool.QueryRow(ctx, `
        INSERT INTO deploys(
            tenant_id, project_id, execution_id, blueprint_id,
            target, environment, status, diff_hash, artifact_hash,
            gate_summary, cost_usd, metadata
        ) VALUES ($1, $2, $3, $4, $5, $6, 'planned', $7, $8, $9::jsonb, $10::numeric, $11::jsonb)
        RETURNING id, created_at`,
		in.TenantID, in.ProjectID, execID, bpID,
		string(in.Target), string(in.Environment),
		in.DiffHash, artifactHashFromMetadata(in),
		string(gateJSON), planRes.EstimatedCostUSD.String(), string(metaJSON),
	)
	if err := row.Scan(&id, &createdAt); err != nil {
		return Deploy{}, fmt.Errorf("deploy: insert: %w", err)
	}

	s.writeEvent(ctx, id, EventPlanned, map[string]any{
		"target":              string(in.Target),
		"environment":         string(in.Environment),
		"provider_project_id": planRes.ProviderProjectID,
	})
	return s.Get(ctx, id)
}

// BuildPreview drives the provider build.
func (s *PostgresService) BuildPreview(ctx context.Context, deployID string) (Deploy, error) {
	d, err := s.Get(ctx, deployID)
	if err != nil {
		return Deploy{}, err
	}
	if d.Status != StatusPlanned && d.Status != StatusFailed {
		return Deploy{}, fmt.Errorf("%w: cannot build preview from %s", ErrInvalidState, d.Status)
	}
	adapter, ok := s.adapter[d.Target]
	if !ok {
		return Deploy{}, fmt.Errorf("%w: %s", ErrUnknownTarget, d.Target)
	}

	if err := s.setStatus(ctx, deployID, StatusPreviewBuilding); err != nil {
		return Deploy{}, err
	}
	s.writeEvent(ctx, deployID, EventPreviewBuilding, nil)

	previewCtx := WithTenant(WithProject(ctx, d.ProjectID), d.TenantID)
	plan := PlanResult{
		ProviderProjectID: deriveProviderProjectIDValue(d),
		EstimatedCostUSD:  d.CostUSD,
	}
	res, err := adapter.BuildPreview(previewCtx, deployID, plan)
	if err != nil {
		_ = s.setStatus(ctx, deployID, StatusFailed)
		s.writeEvent(ctx, deployID, EventFailed, map[string]any{"phase": "preview", "error": err.Error()})
		return Deploy{}, fmt.Errorf("deploy: build preview: %w", err)
	}

	now := time.Now().UTC()
	if _, err := s.pool.Exec(ctx, `
        UPDATE deploys
           SET status                  = 'preview_ready',
               provider_deployment_id  = $2,
               preview_url             = $3,
               preview_ready_at        = $4,
               cost_usd                = cost_usd + $5::numeric
         WHERE id = $1`,
		deployID, res.ProviderDeploymentID, res.PreviewURL, now, res.CostUSD.String(),
	); err != nil {
		return Deploy{}, fmt.Errorf("deploy: update preview: %w", err)
	}

	s.writeEvent(ctx, deployID, EventPreviewReady, map[string]any{
		"provider_deployment_id": res.ProviderDeploymentID,
		"preview_url":            res.PreviewURL,
		"cost_usd":               res.CostUSD.String(),
	})
	return s.Get(ctx, deployID)
}

// RequestApproval opens a pending approval row.
func (s *PostgresService) RequestApproval(ctx context.Context, deployID string, by UserRef, expiresIn time.Duration) (Approval, error) {
	d, err := s.Get(ctx, deployID)
	if err != nil {
		return Approval{}, err
	}
	if d.Status != StatusPreviewReady && d.Status != StatusAwaitingApproval {
		return Approval{}, fmt.Errorf("%w: cannot request approval from %s", ErrInvalidState, d.Status)
	}
	now := time.Now().UTC()
	expiresAt := now.Add(defaultIfZero(expiresIn, s.cfg.DefaultApprovalTTL))
	gateJSON, _ := json.Marshal(stringMapOrEmpty(d.GateSummary))
	reqBy := nullStringIfEmpty(by.UserID)

	var id string
	row := s.pool.QueryRow(ctx, `
        INSERT INTO deploy_approvals(
            deploy_id, tenant_id, requested_by_user_id,
            status, diff_hash, artifact_hash,
            gate_summary, cost_impact_usd, expires_at, requested_at
        ) VALUES ($1, $2, $3, 'pending', $4, $5, $6::jsonb, $7::numeric, $8, $9)
        RETURNING id`,
		deployID, d.TenantID, reqBy,
		d.DiffHash, d.ArtifactHash,
		string(gateJSON), d.CostUSD.String(), expiresAt, now,
	)
	if err := row.Scan(&id); err != nil {
		return Approval{}, fmt.Errorf("deploy: insert approval: %w", err)
	}
	if _, err := s.pool.Exec(ctx, `
        UPDATE deploys SET status = 'awaiting_approval'
         WHERE id = $1 AND status IN ('preview_ready','awaiting_approval')`, deployID); err != nil {
		return Approval{}, fmt.Errorf("deploy: mark awaiting_approval: %w", err)
	}
	s.writeEvent(ctx, deployID, EventApprovalRequest, map[string]any{
		"approval_id": id,
		"expires_at":  expiresAt.Format(time.RFC3339),
	})
	return s.getApproval(ctx, id)
}

// Decide flips an approval row.
func (s *PostgresService) Decide(ctx context.Context, approvalID string, by UserRef, decision string, note string) (Approval, error) {
	verb, ok := normalizeDecision(decision)
	if !ok {
		return Approval{}, fmt.Errorf("%w: unknown decision %q", ErrInvalidState, decision)
	}
	a, err := s.getApproval(ctx, approvalID)
	if err != nil {
		return Approval{}, err
	}
	if a.Status != ApprovalPending {
		return Approval{}, ErrApprovalNotPending
	}
	now := time.Now().UTC()
	if !a.ExpiresAt.IsZero() && a.ExpiresAt.Before(now) {
		_, _ = s.pool.Exec(ctx, `
            UPDATE deploy_approvals SET status = 'expired', decided_at = $2
             WHERE id = $1 AND status = 'pending'`, approvalID, now)
		s.writeEvent(ctx, a.DeployID, EventApprovalExpired, map[string]any{"approval_id": approvalID})
		return Approval{}, ErrApprovalExpired
	}

	newStatus := string(ApprovalApproved)
	if verb == DecisionReject {
		newStatus = string(ApprovalRejected)
	}
	decidedBy := nullStringIfEmpty(by.UserID)
	if _, err := s.pool.Exec(ctx, `
        UPDATE deploy_approvals
           SET status = $2, decided_by_user_id = $3, decision_note = $4, decided_at = $5
         WHERE id = $1 AND status = 'pending'`,
		approvalID, newStatus, decidedBy, note, now,
	); err != nil {
		return Approval{}, fmt.Errorf("deploy: update approval: %w", err)
	}
	if verb == DecisionReject {
		if _, err := s.pool.Exec(ctx, `
            UPDATE deploys SET status = 'cancelled'
             WHERE id = $1 AND status = 'awaiting_approval'`, a.DeployID); err != nil {
			return Approval{}, fmt.Errorf("deploy: cancel on reject: %w", err)
		}
	}
	s.writeEvent(ctx, a.DeployID, EventApprovalDecided, map[string]any{
		"approval_id": approvalID,
		"decision":    verb,
		"note":        note,
	})
	if verb == DecisionReject {
		s.writeEvent(ctx, a.DeployID, EventCancelled, map[string]any{"reason": "approval_rejected"})
	}
	return s.getApproval(ctx, approvalID)
}

// Promote drives the production promote.
func (s *PostgresService) Promote(ctx context.Context, deployID string) (Deploy, error) {
	d, err := s.Get(ctx, deployID)
	if err != nil {
		return Deploy{}, err
	}
	if d.Status != StatusPreviewReady && d.Status != StatusAwaitingApproval {
		return Deploy{}, fmt.Errorf("%w: cannot promote from %s", ErrInvalidState, d.Status)
	}
	adapter, ok := s.adapter[d.Target]
	if !ok {
		return Deploy{}, fmt.Errorf("%w: %s", ErrUnknownTarget, d.Target)
	}
	if d.Environment == EnvironmentProduction {
		if err := GuardDeploy(ctx, s.pg, d.Metadata, string(d.Environment)); err != nil {
			s.writeEvent(ctx, deployID, EventProfitGuardBlock, map[string]any{"error": err.Error()})
			return Deploy{}, err
		}
		rows, err := s.approvalsForDeploy(ctx, deployID)
		if err != nil {
			return Deploy{}, err
		}
		latest := pickLatestApproval(rows)
		if !canPromote(latest, time.Now().UTC()) {
			return Deploy{}, ErrApprovalRequired
		}
	}

	if err := s.setStatus(ctx, deployID, StatusPromoting); err != nil {
		return Deploy{}, err
	}
	s.writeEvent(ctx, deployID, EventPromoting, nil)

	promoteCtx := WithTenant(WithProject(ctx, d.ProjectID), d.TenantID)
	res, err := adapter.Promote(promoteCtx, deployID, d.ProviderDeploymentID)
	if err != nil {
		_ = s.setStatus(ctx, deployID, StatusFailed)
		s.writeEvent(ctx, deployID, EventFailed, map[string]any{"phase": "promote", "error": err.Error()})
		return Deploy{}, fmt.Errorf("deploy: promote: %w", err)
	}
	now := time.Now().UTC()
	if _, err := s.pool.Exec(ctx, `
        UPDATE deploys
           SET status         = 'promoted',
               production_url = $2,
               promoted_at    = $3,
               cost_usd       = cost_usd + $4::numeric
         WHERE id = $1`,
		deployID, res.ProductionURL, now, res.CostUSD.String(),
	); err != nil {
		return Deploy{}, fmt.Errorf("deploy: update promoted: %w", err)
	}
	s.writeEvent(ctx, deployID, EventPromoted, map[string]any{
		"production_url": res.ProductionURL,
		"cost_usd":       res.CostUSD.String(),
	})
	return s.Get(ctx, deployID)
}

// Rollback drives a production rollback.
func (s *PostgresService) Rollback(ctx context.Context, deployID, reason string) (Deploy, error) {
	d, err := s.Get(ctx, deployID)
	if err != nil {
		return Deploy{}, err
	}
	if d.Status != StatusPromoted {
		return Deploy{}, fmt.Errorf("%w: cannot rollback from %s", ErrInvalidState, d.Status)
	}
	adapter, ok := s.adapter[d.Target]
	if !ok {
		return Deploy{}, fmt.Errorf("%w: %s", ErrUnknownTarget, d.Target)
	}
	rollCtx := WithTenant(WithProject(ctx, d.ProjectID), d.TenantID)
	res, err := adapter.Rollback(rollCtx, deployID, d.ProviderDeploymentID, "")
	if err != nil {
		s.writeEvent(ctx, deployID, EventFailed, map[string]any{"phase": "rollback", "error": err.Error()})
		return Deploy{}, fmt.Errorf("deploy: rollback: %w", err)
	}
	now := time.Now().UTC()
	if _, err := s.pool.Exec(ctx, `
        UPDATE deploys
           SET status = 'rolled_back', rolled_back_at = $2
         WHERE id = $1`, deployID, now); err != nil {
		return Deploy{}, fmt.Errorf("deploy: update rolled_back: %w", err)
	}
	s.writeEvent(ctx, deployID, EventRolledBack, map[string]any{
		"to_version": res.ToVersion,
		"reason":     reason,
	})
	return s.Get(ctx, deployID)
}

// Cancel terminates a non-promoted deploy.
func (s *PostgresService) Cancel(ctx context.Context, deployID, reason string) (Deploy, error) {
	d, err := s.Get(ctx, deployID)
	if err != nil {
		return Deploy{}, err
	}
	switch d.Status {
	case StatusPromoted:
		return Deploy{}, fmt.Errorf("%w: cannot cancel promoted deploy (use rollback)", ErrInvalidState)
	case StatusRolledBack, StatusCancelled, StatusFailed:
		return d, nil
	}
	if _, err := s.pool.Exec(ctx, `
        UPDATE deploys SET status = 'cancelled' WHERE id = $1`, deployID); err != nil {
		return Deploy{}, fmt.Errorf("deploy: update cancelled: %w", err)
	}
	s.writeEvent(ctx, deployID, EventCancelled, map[string]any{"reason": reason})
	return s.Get(ctx, deployID)
}

// GetByExecution returns the most recent deploy tagged with the
// given executionID. Returns (zero, false, nil) when no row matches.
func (s *PostgresService) GetByExecution(ctx context.Context, executionID string) (Deploy, bool, error) {
	if executionID == "" {
		return Deploy{}, false, nil
	}
	row := s.pool.QueryRow(ctx, deploySelect()+`
         WHERE execution_id = $1
         ORDER BY created_at DESC
         LIMIT 1`, executionID)
	d, err := scanDeploy(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Deploy{}, false, nil
	}
	if err != nil {
		return Deploy{}, false, err
	}
	return d, true, nil
}

// Get returns a deploy or ErrNotFound.
func (s *PostgresService) Get(ctx context.Context, id string) (Deploy, error) {
	row := s.pool.QueryRow(ctx, deploySelect()+` WHERE id = $1`, id)
	d, err := scanDeploy(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Deploy{}, ErrNotFound
	}
	return d, err
}

// List returns the tenant's recent deploys.
func (s *PostgresService) List(ctx context.Context, tenant string, limit, offset int) ([]Deploy, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.pool.Query(ctx, deploySelect()+`
         WHERE tenant_id = $1
         ORDER BY created_at DESC
         LIMIT $2 OFFSET $3`, tenant, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("deploy: list: %w", err)
	}
	defer rows.Close()
	out := make([]Deploy, 0, limit)
	for rows.Next() {
		d, err := scanDeploy(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// PendingApprovals returns the tenant's open approvals.
func (s *PostgresService) PendingApprovals(ctx context.Context, tenant string) ([]Approval, error) {
	rows, err := s.pool.Query(ctx, approvalSelect()+`
         WHERE tenant_id = $1 AND status = 'pending'
         ORDER BY requested_at DESC`, tenant)
	if err != nil {
		return nil, fmt.Errorf("deploy: pending approvals: %w", err)
	}
	defer rows.Close()
	out := make([]Approval, 0)
	for rows.Next() {
		a, err := scanApproval(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// TenantsWithPendingApprovals returns distinct tenant ids that hold
// at least one pending approval row. Used by the expiry sweeper.
func (s *PostgresService) TenantsWithPendingApprovals(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
        SELECT DISTINCT tenant_id
          FROM deploy_approvals
         WHERE status = 'pending'`)
	if err != nil {
		return nil, fmt.Errorf("deploy: tenants with pending approvals: %w", err)
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		if t == "" {
			continue
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// RecordCost bumps cost_usd and emits an event.
func (s *PostgresService) RecordCost(ctx context.Context, deployID string, addedUSD decimal.Decimal) error {
	if !addedUSD.IsPositive() {
		return nil
	}
	ct, err := s.pool.Exec(ctx, `
        UPDATE deploys SET cost_usd = cost_usd + $2::numeric WHERE id = $1`,
		deployID, addedUSD.String())
	if err != nil {
		return fmt.Errorf("deploy: record cost: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	s.writeEvent(ctx, deployID, EventCostRecorded, map[string]any{"added_usd": addedUSD.String()})
	return nil
}

// SubscribeEvents replays history then fans-out in-process events.
func (s *PostgresService) SubscribeEvents(ctx context.Context, deployID string) (<-chan Event, error) {
	if _, err := s.Get(ctx, deployID); err != nil {
		return nil, err
	}
	history, err := s.listEvents(ctx, deployID)
	if err != nil {
		return nil, err
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

func (s *PostgresService) setStatus(ctx context.Context, deployID string, status Status) error {
	if _, err := s.pool.Exec(ctx, `
        UPDATE deploys SET status = $2 WHERE id = $1`, deployID, string(status)); err != nil {
		return fmt.Errorf("deploy: set status: %w", err)
	}
	return nil
}

func (s *PostgresService) approvalsForDeploy(ctx context.Context, deployID string) ([]Approval, error) {
	rows, err := s.pool.Query(ctx, approvalSelect()+`
         WHERE deploy_id = $1
         ORDER BY requested_at DESC`, deployID)
	if err != nil {
		return nil, fmt.Errorf("deploy: approvals for deploy: %w", err)
	}
	defer rows.Close()
	out := make([]Approval, 0)
	for rows.Next() {
		a, err := scanApproval(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *PostgresService) getApproval(ctx context.Context, id string) (Approval, error) {
	row := s.pool.QueryRow(ctx, approvalSelect()+` WHERE id = $1`, id)
	a, err := scanApproval(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Approval{}, ErrNotFound
	}
	return a, err
}

func (s *PostgresService) writeEvent(ctx context.Context, deployID, eventType string, payload map[string]any) {
	if payload == nil {
		payload = map[string]any{}
	}
	body, _ := json.Marshal(payload)
	var ev Event
	var id int64
	row := s.pool.QueryRow(ctx, `
        INSERT INTO deploy_events(deploy_id, event_type, payload)
        VALUES ($1, $2, $3::jsonb)
        RETURNING id, created_at`, deployID, eventType, string(body))
	var created time.Time
	if err := row.Scan(&id, &created); err != nil {
		s.log.Warn().Err(err).Str("deploy_id", deployID).Str("event_type", eventType).Msg("deploy: write event")
		return
	}
	ev = Event{
		DeployID:  deployID,
		EventType: eventType,
		Payload:   payload,
		CreatedAt: created,
	}
	s.fanout(deployID, ev)
}

func (s *PostgresService) fanout(deployID string, ev Event) {
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
		}
	}
}

func (s *PostgresService) listEvents(ctx context.Context, deployID string) ([]Event, error) {
	rows, err := s.pool.Query(ctx, `
        SELECT event_type, COALESCE(payload::text, '{}'), created_at
        FROM deploy_events
        WHERE deploy_id = $1
        ORDER BY id ASC`, deployID)
	if err != nil {
		return nil, fmt.Errorf("deploy: list events: %w", err)
	}
	defer rows.Close()
	out := make([]Event, 0)
	for rows.Next() {
		var ev Event
		var payloadStr string
		if err := rows.Scan(&ev.EventType, &payloadStr, &ev.CreatedAt); err != nil {
			return nil, err
		}
		var payload map[string]any
		_ = json.Unmarshal([]byte(payloadStr), &payload)
		ev.DeployID = deployID
		ev.Payload = payload
		out = append(out, ev)
	}
	// guard against scanner ordering quirks
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, rows.Err()
}

// ---- scan helpers -------------------------------------------------

func deploySelect() string {
	return `SELECT
            id, tenant_id, project_id, COALESCE(execution_id::text, ''),
            COALESCE(blueprint_id, ''),
            target, environment, status,
            COALESCE(provider_deployment_id, ''),
            COALESCE(preview_url, ''),
            COALESCE(production_url, ''),
            COALESCE(diff_hash, ''),
            COALESCE(artifact_hash, ''),
            COALESCE(gate_summary::text, '{}'),
            cost_usd::text,
            COALESCE(metadata::text, '{}'),
            created_at, preview_ready_at, promoted_at, rolled_back_at
        FROM deploys`
}

func scanDeploy(row pgx.Row) (Deploy, error) {
	var d Deploy
	var target, env, status string
	var gateStr, metaStr, costStr string
	var previewAt, promotedAt, rolledAt *time.Time
	if err := row.Scan(
		&d.ID, &d.TenantID, &d.ProjectID, &d.ExecutionID,
		&d.BlueprintID,
		&target, &env, &status,
		&d.ProviderDeploymentID,
		&d.PreviewURL,
		&d.ProductionURL,
		&d.DiffHash,
		&d.ArtifactHash,
		&gateStr,
		&costStr,
		&metaStr,
		&d.CreatedAt, &previewAt, &promotedAt, &rolledAt,
	); err != nil {
		return Deploy{}, err
	}
	d.Target = Target(target)
	d.Environment = Environment(env)
	d.Status = Status(status)
	d.PreviewReadyAt = previewAt
	d.PromotedAt = promotedAt
	d.RolledBackAt = rolledAt

	cost, err := decimal.NewFromString(costStr)
	if err != nil {
		return Deploy{}, fmt.Errorf("deploy: parse cost_usd %q: %w", costStr, err)
	}
	d.CostUSD = cost

	gates := map[string]any{}
	_ = json.Unmarshal([]byte(gateStr), &gates)
	d.GateSummary = map[string]string{}
	for k, v := range gates {
		if s, ok := v.(string); ok {
			d.GateSummary[k] = s
		} else {
			d.GateSummary[k] = fmt.Sprintf("%v", v)
		}
	}
	meta := map[string]any{}
	_ = json.Unmarshal([]byte(metaStr), &meta)
	d.Metadata = meta
	return d, nil
}

func approvalSelect() string {
	return `SELECT
            id, deploy_id, tenant_id,
            COALESCE(requested_by_user_id::text, ''),
            COALESCE(decided_by_user_id::text, ''),
            status, diff_hash, artifact_hash,
            COALESCE(gate_summary::text, '{}'),
            cost_impact_usd::text,
            expires_at,
            COALESCE(decision_note, ''),
            COALESCE(policy_decision_id, ''),
            COALESCE(audit_chain_event_id, ''),
            requested_at, decided_at
        FROM deploy_approvals`
}

func scanApproval(row pgx.Row) (Approval, error) {
	var a Approval
	var status, costStr, gateStr string
	var decidedAt *time.Time
	if err := row.Scan(
		&a.ID, &a.DeployID, &a.TenantID,
		&a.RequestedByUserID,
		&a.DecidedByUserID,
		&status, &a.DiffHash, &a.ArtifactHash,
		&gateStr,
		&costStr,
		&a.ExpiresAt,
		&a.DecisionNote,
		&a.PolicyDecisionID,
		&a.AuditChainEventID,
		&a.RequestedAt, &decidedAt,
	); err != nil {
		return Approval{}, err
	}
	a.Status = ApprovalStatus(status)
	a.DecidedAt = decidedAt
	cost, err := decimal.NewFromString(costStr)
	if err != nil {
		return Approval{}, fmt.Errorf("deploy: parse cost_impact_usd %q: %w", costStr, err)
	}
	a.CostImpactUSD = cost
	gates := map[string]any{}
	_ = json.Unmarshal([]byte(gateStr), &gates)
	a.GateSummary = map[string]string{}
	for k, v := range gates {
		if s, ok := v.(string); ok {
			a.GateSummary[k] = s
		} else {
			a.GateSummary[k] = fmt.Sprintf("%v", v)
		}
	}
	return a, nil
}

func stringMapOrEmpty(in map[string]string) map[string]string {
	if in == nil {
		return map[string]string{}
	}
	return in
}

func anyMapOrEmpty(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	return in
}

func nullStringIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func deriveProviderProjectIDValue(d Deploy) string {
	if id := metaString(d.Metadata, "vercel_project_id"); id != "" {
		return id
	}
	if id := metaString(d.Metadata, "vercel_project_name"); id != "" {
		return id
	}
	return fmt.Sprintf("ironflyer-%s", shortHash(d.ProjectID))
}
