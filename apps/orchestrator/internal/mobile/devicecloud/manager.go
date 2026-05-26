package devicecloud

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"ironflyer/apps/orchestrator/internal/ledger"
)

// Manager fans device-cloud calls out across the registered providers
// and anchors every session to the wallet via the ledger. Per-user
// isolation: every StartInteractiveSession / EndSession takes projectID
// + workspaceID and stamps them into the ledger metadata so the cost
// dashboards split spend per project, never per platform aggregate.
type Manager struct {
	mu        sync.RWMutex
	providers map[Provider]ProviderClient
	starts    map[string]time.Time // sessionID -> startedAt for billable-min accounting
	ledger    ledger.Service
	logger    zerolog.Logger
}

// New creates an empty Manager. Register providers individually so
// startup can degrade gracefully when only one vendor's credentials are
// wired.
func New(logger zerolog.Logger, ledgerSvc ledger.Service) *Manager {
	return &Manager{
		providers: make(map[Provider]ProviderClient),
		starts:    make(map[string]time.Time),
		ledger:    ledgerSvc,
		logger:    logger.With().Str("component", "devicecloud").Logger(),
	}
}

// Register attaches a ProviderClient under its Name(). Calling twice
// with the same provider overwrites the previous registration — useful
// when credentials rotate without a restart.
func (m *Manager) Register(p ProviderClient) {
	if p == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[p.Name()] = p
	m.logger.Info().Str("provider", string(p.Name())).Msg("device-cloud provider registered")
}

// Providers returns the set of currently registered provider names —
// used by the resolver to render the enabled/disabled chip set.
func (m *Manager) Providers() []Provider {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Provider, 0, len(m.providers))
	for k := range m.providers {
		out = append(out, k)
	}
	return out
}

// ListAllDevices fans the request out across every registered provider
// and concatenates the catalogues. A provider error is logged and
// skipped so a single 502 from BrowserStack doesn't drain the picker.
func (m *Manager) ListAllDevices(ctx context.Context, platform string) ([]Device, error) {
	m.mu.RLock()
	providers := make([]ProviderClient, 0, len(m.providers))
	for _, p := range m.providers {
		providers = append(providers, p)
	}
	m.mu.RUnlock()

	if len(providers) == 0 {
		return nil, ErrProviderNotConfigured
	}

	var out []Device
	for _, p := range providers {
		devs, err := p.ListDevices(ctx, platform)
		if err != nil {
			m.logger.Warn().Err(err).Str("provider", string(p.Name())).
				Msg("device-cloud: list devices failed")
			continue
		}
		out = append(out, devs...)
	}
	return out, nil
}

// StartInteractiveSession allocates a session against the chosen
// provider and writes a ledger entry. The provider is responsible for
// the actual API call; the Manager owns billing accounting + logging.
//
// The opening ledger row records a single minute up-front so the user
// sees the cost line immediately even before the first poll completes.
// The full duration is reconciled in EndSession.
func (m *Manager) StartInteractiveSession(
	ctx context.Context,
	tenantID uuid.UUID,
	projectID, workspaceID string,
	req StartSessionRequest,
	provider Provider,
) (*Session, error) {
	m.mu.RLock()
	client, ok := m.providers[provider]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrProviderNotConfigured, provider)
	}

	session, err := client.StartSession(ctx, req)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, fmt.Errorf("devicecloud: %s returned nil session", provider)
	}

	m.mu.Lock()
	m.starts[session.ID] = session.StartedAt
	m.mu.Unlock()

	if m.ledger != nil {
		if _, lerr := ledger.RecordDeviceCloudMinutes(ctx, m.ledger, tenantID, uuid.Nil, ledger.DeviceCloudMeta{
			Provider:    string(provider),
			SessionID:   session.ID,
			DeviceID:    session.DeviceID,
			ProjectID:   projectID,
			WorkspaceID: workspaceID,
			Phase:       "start",
		}, 1); lerr != nil {
			m.logger.Warn().Err(lerr).Str("session_id", session.ID).
				Msg("device-cloud: ledger write at start failed")
		}
	}

	m.logger.Info().
		Str("provider", string(provider)).
		Str("session_id", session.ID).
		Str("device_id", session.DeviceID).
		Str("project_id", projectID).
		Str("workspace_id", workspaceID).
		Msg("device-cloud session started")
	return session, nil
}

// EndSession terminates the underlying provider session and writes the
// reconciliation ledger entry covering the remaining billable minutes.
func (m *Manager) EndSession(
	ctx context.Context,
	tenantID uuid.UUID,
	projectID, workspaceID, sessionID string,
	provider Provider,
) error {
	m.mu.RLock()
	client, ok := m.providers[provider]
	startedAt := m.starts[sessionID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("%w: %s", ErrProviderNotConfigured, provider)
	}

	if err := client.EndSession(ctx, sessionID); err != nil {
		return err
	}

	m.mu.Lock()
	delete(m.starts, sessionID)
	m.mu.Unlock()

	// Reconcile billable minutes — we charged 1 minute up-front at
	// start; charge the delta now so the ledger reflects actual usage.
	if m.ledger != nil && !startedAt.IsZero() {
		elapsed := time.Since(startedAt).Minutes()
		// Round up so the 5s session that ran a real device still
		// pays for the first minute we already booked.
		remaining := elapsed - 1
		if remaining > 0 {
			if _, lerr := ledger.RecordDeviceCloudMinutes(ctx, m.ledger, tenantID, uuid.Nil, ledger.DeviceCloudMeta{
				Provider:    string(provider),
				SessionID:   sessionID,
				ProjectID:   projectID,
				WorkspaceID: workspaceID,
				Phase:       "end",
			}, remaining); lerr != nil {
				m.logger.Warn().Err(lerr).Str("session_id", sessionID).
					Msg("device-cloud: ledger write at end failed")
			}
		}
	}

	m.logger.Info().
		Str("provider", string(provider)).
		Str("session_id", sessionID).
		Str("project_id", projectID).
		Str("workspace_id", workspaceID).
		Msg("device-cloud session ended")
	return nil
}
