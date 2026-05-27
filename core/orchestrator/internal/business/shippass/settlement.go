package shippass

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"ironflyer/core/orchestrator/internal/ai/domain"
)

// Settler subscribes the Ship Pass lifecycle to the existing finisher
// gate verdict stream. The orchestrator already publishes gate
// verdicts on its SSE bus and into the learning outcome stream; the
// Settler is the bridge that translates "gate X verdict on project Y"
// into "RecordGateVerdict on the active pass for Y".
//
// The integration is deliberately push-based: wireup hands the
// orchestrator's gate-verdict callback a closure that calls
// Settler.Observe. The Settler holds no state of its own beyond a
// short-lived (projectID → passID) cache so it does not have to scan
// every active pass on every verdict — the cache miss path falls
// through to ActiveForProject and refills.
type Settler struct {
	svc Service
	log zerolog.Logger

	mu    sync.RWMutex
	cache map[string]string // projectID → active passID
}

// NewSettler constructs a Settler over the Service. The logger is
// optional — passing a zero value disables logging without panicking.
func NewSettler(svc Service, log zerolog.Logger) *Settler {
	return &Settler{
		svc:   svc,
		log:   log,
		cache: map[string]string{},
	}
}

// Observe is called by the orchestrator gate verdict emitter. The
// signature is intentionally tenant-free — the Settler resolves the
// tenant from the active pass — so the caller (which holds projectID
// + gate name + verdict) never has to thread tenant context through
// every gate hook.
//
// observedAt should be the verdict's authoritative timestamp; pass
// time.Now() when the emitter has no better signal.
func (s *Settler) Observe(ctx context.Context, projectID string, gate domain.GateName, passed bool, reason string, observedAt time.Time) {
	if projectID == "" || gate == "" {
		return
	}
	passID, ok := s.lookup(projectID)
	if !ok {
		// We don't know the tenant at the Observe call site. Walk a
		// nil tenant through ActiveForProject would leak data, so we
		// instead expose a separate "Bind" method (Bind) that resolver
		// purchase wires at Purchase time. On a cold cache the Settler
		// no-ops — the next purchase or the periodic Rebind tick fills
		// it.
		s.log.Debug().
			Str("project_id", projectID).
			Str("gate", string(gate)).
			Msg("shippass settler: no active pass cached, skipping verdict")
		return
	}
	if _, err := s.svc.RecordGateVerdict(ctx, passID, gate, passed, reason, observedAt); err != nil {
		s.log.Warn().Err(err).
			Str("pass_id", passID).
			Str("gate", string(gate)).
			Msg("shippass settler: record verdict failed")
	}
}

// Bind registers a pass id against a project id so subsequent
// Observe calls can route without an ActiveForProject lookup. Called
// by the Purchase resolver as part of the buy flow.
func (s *Settler) Bind(projectID, passID string) {
	if projectID == "" || passID == "" {
		return
	}
	s.mu.Lock()
	s.cache[projectID] = passID
	s.mu.Unlock()
}

// Unbind forgets a project's pass id mapping. Called by the lifecycle
// hooks when a pass leaves the active state (shipped, refunded,
// cancelled) so a follow-up project that briefly shares the same
// project id (rare; copies) does not see stale routing.
func (s *Settler) Unbind(projectID string) {
	if projectID == "" {
		return
	}
	s.mu.Lock()
	delete(s.cache, projectID)
	s.mu.Unlock()
}

// Rebind walks the Service's recent passes and refills the cache. The
// cron caller invokes this periodically so a process restart does
// not lose the routing table for in-flight passes.
func (s *Settler) Rebind(ctx context.Context, tenants []string) error {
	for _, tenant := range tenants {
		rows, err := s.svc.List(ctx, tenant, 100)
		if err != nil {
			s.log.Warn().Err(err).Str("tenant", tenant).Msg("shippass settler: rebind list failed")
			continue
		}
		for _, p := range rows {
			if p.Status == StatusActive {
				s.Bind(p.ProjectID, p.ID)
			}
		}
	}
	return nil
}

// lookup returns the cached pass id for projectID, if any.
func (s *Settler) lookup(projectID string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.cache[projectID]
	return id, ok
}
