package session

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// idleEviction bounds how long a session can sit without producing a
// new H.264 frame before the reaper terminates it. Five minutes is a
// generous human attention span and a tight bound on stuck scrcpy
// processes.
const idleEviction = 5 * time.Minute

// reaperInterval is how often the janitor scans the live sessions for
// idle eviction candidates.
const reaperInterval = 30 * time.Second

// Manager owns the lifecycle of every bridge session in this process.
// It enforces per-workspace isolation via byWorkspace and runs the
// idle reaper in the background.
type Manager struct {
	logger zerolog.Logger

	scrcpyPath string
	adbServer  string

	mu          sync.RWMutex
	sessions    map[string]*Session
	byWorkspace map[string][]string

	stopReaper chan struct{}
}

// NewManager returns a Manager with the reaper already started. Call
// Shutdown to drain.
func NewManager(scrcpyPath, adbServer string, logger zerolog.Logger) *Manager {
	m := &Manager{
		logger:      logger.With().Str("component", "session-manager").Logger(),
		scrcpyPath:  scrcpyPath,
		adbServer:   adbServer,
		sessions:    map[string]*Session{},
		byWorkspace: map[string][]string{},
		stopReaper:  make(chan struct{}),
	}
	go m.reapLoop()
	return m
}

// Create allocates a Session and starts scrcpy. Errors leave no
// state behind — callers can retry safely.
func (m *Manager) Create(ctx context.Context, workspaceID, emulatorSerial string) (*Session, error) {
	s, err := New(workspaceID, emulatorSerial, m.logger)
	if err != nil {
		return nil, err
	}
	if err := s.Start(ctx, m.scrcpyPath, m.adbServer); err != nil {
		return nil, err
	}
	m.mu.Lock()
	m.sessions[s.ID] = s
	m.byWorkspace[workspaceID] = append(m.byWorkspace[workspaceID], s.ID)
	m.mu.Unlock()
	m.logger.Info().Str("session", s.ID).Str("workspace", workspaceID).
		Msg("session created")
	return s, nil
}

// Get returns the session for id and reports whether it exists.
func (m *Manager) Get(id string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	return s, ok
}

// ListByWorkspace returns a snapshot of session IDs scoped to the
// caller's workspace. Used to enforce per-user isolation in /v1/sessions.
func (m *Manager) ListByWorkspace(workspaceID string) []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := m.byWorkspace[workspaceID]
	out := make([]*Session, 0, len(ids))
	for _, id := range ids {
		if s, ok := m.sessions[id]; ok {
			out = append(out, s)
		}
	}
	return out
}

// Delete closes the session and removes it from the lookup tables.
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	s, ok := m.sessions[id]
	if !ok {
		m.mu.Unlock()
		return errors.New("session not found")
	}
	delete(m.sessions, id)
	m.byWorkspace[s.WorkspaceID] = removeID(m.byWorkspace[s.WorkspaceID], id)
	if len(m.byWorkspace[s.WorkspaceID]) == 0 {
		delete(m.byWorkspace, s.WorkspaceID)
	}
	m.mu.Unlock()
	return s.Close()
}

// Shutdown stops the reaper and closes every live session. Used by
// SIGTERM-driven graceful shutdown.
func (m *Manager) Shutdown() {
	close(m.stopReaper)
	m.mu.Lock()
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		_ = m.Delete(id)
	}
}

func (m *Manager) reapLoop() {
	t := time.NewTicker(reaperInterval)
	defer t.Stop()
	for {
		select {
		case <-m.stopReaper:
			return
		case <-t.C:
			m.reapOnce()
		}
	}
}

func (m *Manager) reapOnce() {
	now := time.Now()
	m.mu.RLock()
	candidates := make([]*Session, 0)
	for _, s := range m.sessions {
		last := s.LastFrameAt()
		if last.IsZero() {
			// Grace window for startup: ignore sessions that
			// haven't pumped a frame yet but are younger than
			// the idle threshold.
			if now.Sub(s.StartedAt()) < idleEviction {
				continue
			}
			candidates = append(candidates, s)
			continue
		}
		if now.Sub(last) > idleEviction {
			candidates = append(candidates, s)
		}
	}
	m.mu.RUnlock()
	for _, s := range candidates {
		m.logger.Info().Str("session", s.ID).Msg("evicting idle session")
		_ = m.Delete(s.ID)
	}
}

func removeID(ids []string, id string) []string {
	out := ids[:0]
	for _, v := range ids {
		if v != id {
			out = append(out, v)
		}
	}
	return out
}
