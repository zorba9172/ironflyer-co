package completion

import (
	"context"
	"sort"
	"sync"
	"time"
)

// memoryState keeps the raw per-execution latest-pass-by-gate map plus
// the append-only event log. We need the raw map to recompute the
// score correctly on the next Score(...) call; reconstructing it from
// the event log alone is lossy.
type memoryState struct {
	latestByGate map[string]bool
	events       []ScoreEvent
}

// MemoryScorer is the in-memory Scorer implementation used by tests,
// the mock driver, and any environment without Postgres wired in.
//
// All state is kept under a single mutex; the workload is small (one
// scorer is shared across a process and the per-execution history is
// short) so finer-grained locking is not worth the complexity.
type MemoryScorer struct {
	mu    sync.Mutex
	state map[string]*memoryState
}

// NewMemoryScorer returns a ready-to-use in-memory scorer.
func NewMemoryScorer() *MemoryScorer {
	return &MemoryScorer{state: make(map[string]*memoryState)}
}

// Score appends the gate outcome, recomputes the absolute score, and
// returns the new score plus the delta from the previous absolute
// score.
func (m *MemoryScorer) Score(_ context.Context, executionID string, outcome GateOutcome) (float64, float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	st, ok := m.state[executionID]
	if !ok {
		st = &memoryState{latestByGate: map[string]bool{}}
		m.state[executionID] = st
	}
	previous := 0.0
	if n := len(st.events); n > 0 {
		previous = st.events[n-1].Score
	}

	st.latestByGate[outcome.Gate] = outcome.Passed
	newScore := computeScore(st.latestByGate)
	delta := newScore - previous

	st.events = append(st.events, ScoreEvent{
		Gate:       outcome.Gate,
		Score:      newScore,
		Delta:      delta,
		RecordedAt: time.Now().UTC(),
	})
	return newScore, delta, nil
}

// Get returns the latest absolute score for the execution (0 if none).
func (m *MemoryScorer) Get(_ context.Context, executionID string) (float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st, ok := m.state[executionID]
	if !ok || len(st.events) == 0 {
		return 0, nil
	}
	return st.events[len(st.events)-1].Score, nil
}

// History returns the recorded events in chronological order.
func (m *MemoryScorer) History(_ context.Context, executionID string) ([]ScoreEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st, ok := m.state[executionID]
	if !ok {
		return nil, nil
	}
	out := make([]ScoreEvent, len(st.events))
	copy(out, st.events)
	sort.Slice(out, func(i, j int) bool { return out[i].RecordedAt.Before(out[j].RecordedAt) })
	return out, nil
}
