package guild

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/ai/learning"
)

// Coordinator is the resolver-facing facade. It folds the Service,
// Escrow, Payouts, and TemplateRegistry into one entry-point so the
// GraphQL layer does not have to juggle four dependencies. Every
// mutation here:
//
//   - Checks ownership (the resolver passes the caller's user id +
//     tenant; we trust those values because they came from the auth
//     middleware).
//   - Runs the wallet move via Escrow.
//   - Records the idempotent op via Service.RecordOp.
//   - Emits a learning.OutcomeEvent so the Feedback Brain can mine
//     guild patterns.
//
// Coordinator is constructed once at wireup and shared across
// resolver calls — every method takes ctx and is safe for concurrent
// use because the underlying Service implementations are.
type Coordinator struct {
	svc       Service
	escrow    *Escrow
	payouts   *Payouts
	templates *TemplateRegistry
	logger    zerolog.Logger
}

// NewCoordinator builds the facade.
func NewCoordinator(svc Service, escrow *Escrow, payouts *Payouts, templates *TemplateRegistry, logger zerolog.Logger) *Coordinator {
	return &Coordinator{svc: svc, escrow: escrow, payouts: payouts, templates: templates, logger: logger}
}

// CreateTask is the requestor-facing "open a guild task" path. Caller
// has already authorised the project (the resolver runs the owner-
// check); we hold the floor, persist the row, and emit the
// OutcomeEvent.
func (c *Coordinator) CreateTask(ctx context.Context, projectID, tenantID, title, description string, floor decimal.Decimal, slaHours int) (GuildTask, error) {
	if floor.IsZero() || floor.IsNegative() {
		return GuildTask{}, ErrInvalidAmount
	}
	if err := c.escrow.HoldFloor(ctx, tenantID, floor); err != nil {
		return GuildTask{}, err
	}
	task, err := c.svc.CreateTask(ctx, GuildTask{
		ProjectID:     projectID,
		TenantID:      tenantID,
		Title:         title,
		Description:   description,
		PriceUSDFloor: floor,
		SLAHours:      slaHours,
		Status:        TaskStatusOpen,
		CreatedAt:     time.Now().UTC(),
	})
	if err != nil {
		_ = c.escrow.ReleaseFloor(ctx, tenantID, floor)
		return GuildTask{}, err
	}
	learning.Publish(ctx, learning.OutcomeEvent{
		TenantID: tenantID,
		Kind:     learning.OutcomeKind("guild.task.created"),
		Attributes: map[string]any{
			"task_id":    task.ID,
			"project_id": projectID,
			"floor_usd":  floor.String(),
			"sla_hours":  slaHours,
		},
		Success: learning.BoolPtr(true),
	})
	return task, nil
}

// PlaceBid is the finisher-facing bid path. finisherUserID + finisher
// profile resolution is done in the resolver before delegating; we
// re-assert PriceUSD <= floor by funnelling through Service.PlaceBid
// (which holds the row lock in Postgres).
func (c *Coordinator) PlaceBid(ctx context.Context, taskID, finisherID string, price decimal.Decimal, estHours int, note string) (Bid, error) {
	bid, err := c.svc.PlaceBid(ctx, Bid{
		TaskID:         taskID,
		FinisherID:     finisherID,
		PriceUSD:       price,
		EstimatedHours: estHours,
		Note:           note,
		Status:         BidStatusOpen,
		CreatedAt:      time.Now().UTC(),
	})
	if err != nil {
		return Bid{}, err
	}
	learning.Publish(ctx, learning.OutcomeEvent{
		Kind: learning.OutcomeKind("guild.bid.placed"),
		Attributes: map[string]any{
			"bid_id":      bid.ID,
			"task_id":     taskID,
			"finisher_id": finisherID,
			"price_usd":   price.String(),
		},
		Success: learning.BoolPtr(true),
	})
	return bid, nil
}

// AcceptBid is the requestor-facing "pick this finisher" path. The
// resolver authorises (caller owns the project the task belongs to);
// we settle the wallet, mark the bid won, flip the task to accepted,
// queue the payout, and emit the OutcomeEvent.
//
// Idempotent via AcceptBidOpKey — a Temporal retry hitting the same
// bid id short-circuits.
func (c *Coordinator) AcceptBid(ctx context.Context, bidID string) (GuildTask, error) {
	opKey := AcceptBidOpKey(bidID)
	if prior, ok, err := c.svc.RecallOp(ctx, opKey); err != nil {
		return GuildTask{}, err
	} else if ok {
		if prior.Status == "succeeded" {
			bid, err := c.svc.GetBid(ctx, bidID)
			if err != nil {
				return GuildTask{}, err
			}
			return c.svc.GetTask(ctx, bid.TaskID)
		}
		return GuildTask{}, ErrInvalidStatus
	}
	bid, err := c.svc.GetBid(ctx, bidID)
	if err != nil {
		return GuildTask{}, err
	}
	if bid.Status != BidStatusOpen {
		return GuildTask{}, ErrInvalidStatus
	}
	task, err := c.svc.GetTask(ctx, bid.TaskID)
	if err != nil {
		return GuildTask{}, err
	}
	if task.Status != TaskStatusOpen && task.Status != TaskStatusBidding {
		return GuildTask{}, ErrInvalidStatus
	}
	if err := c.escrow.SettleAccepted(ctx, task.TenantID, task.PriceUSDFloor, bid.PriceUSD); err != nil {
		_ = c.svc.RecordOp(ctx, opKey, string(OpAcceptBid), bid.PriceUSD, "failed", err.Error())
		return GuildTask{}, err
	}
	if _, err := c.svc.UpdateBidStatus(ctx, bid.ID, BidStatusWon); err != nil {
		return GuildTask{}, err
	}
	finisherID := bid.FinisherID
	updated, err := c.svc.UpdateTaskStatus(ctx, task.ID, TaskStatusAccepted, &finisherID)
	if err != nil {
		return GuildTask{}, err
	}
	if _, err := c.payouts.QueuePayout(ctx, task.ID, bid.FinisherID, bid.PriceUSD); err != nil {
		c.logger.Warn().Err(err).Str("task_id", task.ID).Msg("guild: payout queue failed")
	}
	_ = c.svc.RecordOp(ctx, opKey, string(OpAcceptBid), bid.PriceUSD, "succeeded", "")
	platformCut, finisherCut := SplitTaskAmount(bid.PriceUSD)
	learning.Publish(ctx, learning.OutcomeEvent{
		TenantID: task.TenantID,
		Kind:     learning.OutcomeKind("guild.bid.accepted"),
		Attributes: map[string]any{
			"task_id":          task.ID,
			"bid_id":           bid.ID,
			"finisher_id":      bid.FinisherID,
			"price_usd":        bid.PriceUSD.String(),
			"finisher_cut_usd": finisherCut.String(),
			"platform_cut_usd": platformCut.String(),
		},
		Success:   learning.BoolPtr(true),
		CostUSD:   learning.DecimalPtr(bid.PriceUSD),
		MarginUSD: learning.DecimalPtr(platformCut),
	})
	return updated, nil
}

// RejectTask transitions the task to 'rejected' and releases the
// requestor's floor hold. Caller must own the project.
func (c *Coordinator) RejectTask(ctx context.Context, taskID, reason string) (GuildTask, error) {
	task, err := c.svc.GetTask(ctx, taskID)
	if err != nil {
		return GuildTask{}, err
	}
	switch task.Status {
	case TaskStatusRejected, TaskStatusAccepted, TaskStatusExpired:
		return GuildTask{}, ErrInvalidStatus
	}
	if err := c.escrow.ReleaseFloor(ctx, task.TenantID, task.PriceUSDFloor); err != nil {
		return GuildTask{}, err
	}
	updated, err := c.svc.UpdateTaskStatus(ctx, task.ID, TaskStatusRejected, nil)
	if err != nil {
		return GuildTask{}, err
	}
	learning.Publish(ctx, learning.OutcomeEvent{
		TenantID: task.TenantID,
		Kind:     learning.OutcomeKind("guild.task.rejected"),
		Attributes: map[string]any{
			"task_id":    task.ID,
			"project_id": task.ProjectID,
			"reason":     reason,
		},
		Success: learning.BoolPtr(false),
	})
	return updated, nil
}

// InstallTemplate routes through the TemplateRegistry; this method is
// a thin convenience so the resolver only depends on *Coordinator.
func (c *Coordinator) InstallTemplate(ctx context.Context, slug, projectID, tenantID string) (Template, error) {
	t, err := c.svc.GetTemplateBySlug(ctx, slug)
	if err != nil {
		return Template{}, err
	}
	return c.templates.Install(ctx, t, projectID, tenantID)
}

// Service returns the underlying store for read-only resolver queries.
// Mutations should go through the Coordinator surface.
func (c *Coordinator) Service() Service { return c.svc }
