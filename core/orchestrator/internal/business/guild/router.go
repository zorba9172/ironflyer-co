package guild

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/ai/learning"
)

// GateFailureRouter watches the gate-verdict stream the finisher
// publishes via learning.OutcomeEvent (Kind=gate_outcome). When a
// configured gate has failed N consecutive times for the same project,
// the router auto-creates a GuildTask in 'open' status so a finisher
// can pick it up.
//
// We deliberately do NOT introduce a new event bus. Instead, the router
// implements an `Observe(learning.OutcomeEvent)` method that wireup
// installs as the publisher's observer (via a fan-out shim — see
// wireup/guild.go). The publisher's existing single-observer slot stays
// occupied by the memory store; the shim multiplexes.
//
// Threshold defaults: 3 consecutive failures (configurable per gate
// via Register). PriceUSDFloor for auto-created tasks defaults to $50
// — low enough that the cron's Hold succeeds on most wallets, high
// enough to attract a finisher. Real per-gate pricing belongs in a
// future Bandit-driven knob.
type GateFailureRouter struct {
	svc          Service
	escrow       *Escrow
	projects     ProjectLookup
	logger       zerolog.Logger
	defaultFloor decimal.Decimal

	mu          sync.Mutex
	thresholds  map[string]int            // gate name -> threshold
	failCounts  map[string]map[string]int // gate -> projectID -> consecutive fails
	openTaskFor map[string]string         // projectID:gate -> taskID (prevents duplicate auto-creates)
}

// ProjectLookup is the narrow interface the router needs to resolve a
// projectID into (tenantID, ownerID). Wireup hands a small adapter
// over store.Store; we keep the surface tight so the router does not
// pull the whole projects store into its import graph.
type ProjectLookup interface {
	TenantForProject(ctx context.Context, projectID string) (tenant string, ok bool)
}

// NewGateFailureRouter builds a router with sane defaults.
func NewGateFailureRouter(svc Service, escrow *Escrow, projects ProjectLookup, logger zerolog.Logger) *GateFailureRouter {
	return &GateFailureRouter{
		svc:          svc,
		escrow:       escrow,
		projects:     projects,
		logger:       logger,
		defaultFloor: decimal.NewFromInt(50),
		thresholds:   map[string]int{},
		failCounts:   map[string]map[string]int{},
		openTaskFor:  map[string]string{},
	}
}

// Register configures the failure threshold for a gate. A second
// Register on the same gate replaces the threshold; threshold <= 0
// disables auto-routing for that gate (the router stays subscribed but
// never fires). Pass `*` as gateName to set a wildcard default.
func (r *GateFailureRouter) Register(gateName string, threshold int) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.thresholds[gateName] = threshold
}

// Observe is the publisher-observer callback. It is fanned out from
// the wireup shim and runs on the publisher's best-effort goroutine —
// errors are logged, never propagated.
func (r *GateFailureRouter) Observe(evt learning.OutcomeEvent) {
	if r == nil {
		return
	}
	if evt.Kind != learning.KindGateOutcome {
		return
	}
	gate, _ := evt.Attributes["gate"].(string)
	verdict, _ := evt.Attributes["verdict"].(string)
	projectID, _ := evt.Attributes["project_id"].(string)
	if gate == "" {
		return
	}
	// projectID is not currently stamped onto gate_outcome attributes
	// — the engine emits gate verdicts without it. Fall back to the
	// tenant id so we still count failures per tenant; when the engine
	// adds project_id (tracked in docs/FEEDBACK_BRAIN.md) the bucket
	// becomes per-project automatically.
	if projectID == "" {
		projectID = evt.TenantID
	}
	if projectID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	switch verdict {
	case "pass":
		// Reset the streak on any pass — gates are not "failed forever"
		// after a single hiccup.
		if m, ok := r.failCounts[gate]; ok {
			delete(m, projectID)
		}
		return
	case "fail":
		// Fall through.
	default:
		return
	}
	threshold := r.thresholds[gate]
	if threshold <= 0 {
		threshold = r.thresholds["*"]
	}
	if threshold <= 0 {
		return
	}
	if r.failCounts[gate] == nil {
		r.failCounts[gate] = map[string]int{}
	}
	r.failCounts[gate][projectID]++
	count := r.failCounts[gate][projectID]
	if count < threshold {
		return
	}
	bucket := projectID + ":" + gate
	if _, exists := r.openTaskFor[bucket]; exists {
		return
	}
	// Hand off to a goroutine so the publisher's fanout latency does
	// not bloat. The router has 5s to materialize the task; failing
	// inside the goroutine logs a warn and resets the counter so a
	// subsequent failure tries again.
	r.openTaskFor[bucket] = "pending"
	go r.materialize(projectID, gate, evt)
}

// materialize creates the GuildTask + emits the OutcomeEvent +
// updates openTaskFor. Runs off the observer goroutine.
func (r *GateFailureRouter) materialize(projectID, gate string, evt learning.OutcomeEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tenant := evt.TenantID
	if r.projects != nil {
		if t, ok := r.projects.TenantForProject(ctx, projectID); ok && t != "" {
			tenant = t
		}
	}
	if tenant == "" {
		r.logger.Warn().Str("gate", gate).Str("project_id", projectID).
			Msg("guild router: tenant lookup failed; not auto-creating task")
		r.clearBucket(projectID, gate)
		return
	}
	if err := r.escrow.HoldFloor(ctx, tenant, r.defaultFloor); err != nil {
		r.logger.Warn().Err(err).
			Str("tenant", tenant).Str("gate", gate).
			Msg("guild router: hold failed; deferring task creation")
		r.clearBucket(projectID, gate)
		return
	}
	title := "Close gate " + gate
	desc := buildAutoTaskDescription(gate, evt)
	gateFailureID, _ := evt.Attributes["gate_failure_id"].(string)
	task, err := r.svc.CreateTask(ctx, GuildTask{
		ProjectID:     projectID,
		TenantID:      tenant,
		GateFailureID: gateFailureID,
		Title:         title,
		Description:   desc,
		PriceUSDFloor: r.defaultFloor,
		SLAHours:      48,
		Status:        TaskStatusOpen,
	})
	if err != nil {
		// Release the hold we just took so the requestor's wallet
		// stays consistent.
		_ = r.escrow.ReleaseFloor(ctx, tenant, r.defaultFloor)
		r.logger.Warn().Err(err).Str("gate", gate).Msg("guild router: task create failed")
		r.clearBucket(projectID, gate)
		return
	}
	r.mu.Lock()
	r.openTaskFor[projectID+":"+gate] = task.ID
	r.failCounts[gate][projectID] = 0
	r.mu.Unlock()
	learning.Publish(ctx, learning.OutcomeEvent{
		TenantID: tenant,
		Kind:     learning.OutcomeKind("guild.task.auto_created"),
		Attributes: map[string]any{
			"task_id":    task.ID,
			"project_id": projectID,
			"gate":       gate,
			"floor_usd":  r.defaultFloor.String(),
		},
		Success: learning.BoolPtr(true),
		Tags:    map[string]string{"gate": gate},
	})
	r.logger.Info().
		Str("task_id", task.ID).
		Str("project_id", projectID).
		Str("gate", gate).
		Msg("guild router: auto-created task after consecutive gate failures")
}

func (r *GateFailureRouter) clearBucket(projectID, gate string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.openTaskFor, projectID+":"+gate)
}

// buildAutoTaskDescription stitches gate name + first issue message
// into a description the finisher can scan in two seconds.
func buildAutoTaskDescription(gate string, evt learning.OutcomeEvent) string {
	var b strings.Builder
	b.WriteString("Auto-routed by Ironflyer after repeated failures on the ")
	b.WriteString(gate)
	b.WriteString(" gate. ")
	if msg, ok := evt.Attributes["last_issue"].(string); ok && msg != "" {
		b.WriteString("Most recent issue: ")
		b.WriteString(msg)
	} else if issues, ok := evt.Attributes["issues"].(int); ok && issues > 0 {
		b.WriteString("Open issues: ")
		b.WriteString(itoa(issues))
	}
	return b.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
