// Package memory is the persistent intelligence moat — Layer 6 of the
// AI Completion Infrastructure blueprint. The longer a project lives,
// the smarter the system becomes; the harder it is to leave; the more
// switching costs accrue. That property does not come from the LLM.
// It comes from this package.
//
// Four orthogonal stores live here. Each one captures a different
// flavour of evidence that the orchestrator and the agents read on
// every run:
//
//   - ProjectMemory   "this project's architecture / conventions /
//                      design rules / module map / roadmap"
//
//   - ExecutionMemory "what failed before, what fixed it"
//                      a failure-→-fix lineage so the recovery agent
//                      sees prior wins instead of re-inventing them.
//
//   - UserMemory      "this user's stylistic choices, the corrections
//                      they keep making, the patterns they reject."
//                      Lets the Coder respect the user's voice across
//                      projects.
//
//   - BusinessMemory  "the goals, KPIs, customer segments, monetisation
//                      strategy" — the why behind the project. Drives
//                      Architecture + Roadmap choices.
//
// The interface is small on purpose. Operators can swap MemoryStore for
// a Postgres / SurrealDB / pgvector implementation; the orchestrator
// only depends on the contract.

package memory

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Kind names the four memory dimensions. The string values are stable —
// they appear on the wire (HTTP / SSE / SDK), in agent prompts, and in
// any external storage backend, so renaming them is a breaking change.
type Kind string

const (
	KindProject   Kind = "project"
	KindExecution Kind = "execution"
	KindUser      Kind = "user"
	KindBusiness  Kind = "business"
)

// AllKinds returns the canonical iteration order. Use it when you want
// to iterate every dimension (e.g. building a Coder context bundle).
func AllKinds() []Kind {
	return []Kind{KindProject, KindExecution, KindUser, KindBusiness}
}

// Record is a single memory entry. The same shape covers all four
// kinds; the discriminator is Kind plus the scoping ids.
//
// Scoping rules (only the ids relevant to Kind are filled):
//
//   - KindProject:   ProjectID required
//   - KindExecution: ProjectID required, optional StoryID / GateName
//   - KindUser:      UserID required (project-agnostic)
//   - KindBusiness:  ProjectID required
//
// Tags are free-form labels callers use to slice records ("decision",
// "convention", "failure", "fix", "kpi", "stack-choice"). Confidence
// is 0..1; the writer is honest about how strong the signal is.
type Record struct {
	ID         string    `json:"id"`
	Kind       Kind      `json:"kind"`
	ProjectID  string    `json:"projectId,omitempty"`
	UserID     string    `json:"userId,omitempty"`
	StoryID    string    `json:"storyId,omitempty"`
	GateName   string    `json:"gateName,omitempty"`
	Title      string    `json:"title"`
	Body       string    `json:"body"`
	Tags       []string  `json:"tags,omitempty"`
	Confidence float64   `json:"confidence,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

// Query is the read filter. Zero-valued fields are wildcards. Limit
// 0 means "use the default" (Store decides; usually 20).
type Query struct {
	Kind      Kind
	ProjectID string
	UserID    string
	StoryID   string
	GateName  string
	Tag       string // single-tag filter; empty = any
	Substring string // matched against Title+Body (case-insensitive)
	Limit     int
}

// Store is the operator-replaceable contract. Implementations MUST be
// safe for concurrent use; the orchestrator hits this from gate +
// repair + chat paths in parallel.
type Store interface {
	Record(ctx context.Context, r Record) (Record, error)
	Query(ctx context.Context, q Query) ([]Record, error)
	Delete(ctx context.Context, id string) error
}

// MemoryStore is the default in-process backend. Bounded ring buffer
// per Kind to keep RAM predictable even on very long-lived servers.
type MemoryStore struct {
	mu     sync.RWMutex
	rows   map[Kind][]Record
	maxPer int
}

// NewMemoryStore returns a fresh in-memory backend with the given
// per-kind cap. Default 4096 entries per kind ≈ a few MiB.
func NewMemoryStore(maxPerKind int) *MemoryStore {
	if maxPerKind <= 0 {
		maxPerKind = 4096
	}
	return &MemoryStore{
		rows:   map[Kind][]Record{},
		maxPer: maxPerKind,
	}
}

// Record persists r. ID + CreatedAt are filled if zero. Returns the
// stored record so callers can attach the assigned id.
func (m *MemoryStore) Record(_ context.Context, r Record) (Record, error) {
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	bucket := m.rows[r.Kind]
	if len(bucket) >= m.maxPer {
		// Evict oldest. Cheap shift; N is bounded by maxPer.
		copy(bucket, bucket[1:])
		bucket = bucket[:len(bucket)-1]
	}
	bucket = append(bucket, r)
	m.rows[r.Kind] = bucket
	return r, nil
}

// Query returns records newest-first that match every set field on q.
// An empty Query returns nothing — callers must scope by at least Kind
// or ProjectID, otherwise the result would be a firehose.
func (m *MemoryStore) Query(_ context.Context, q Query) ([]Record, error) {
	if q.Kind == "" && q.ProjectID == "" && q.UserID == "" {
		return nil, nil
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 20
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	scan := func(rows []Record, out []Record) []Record {
		// Newest-first walk.
		for i := len(rows) - 1; i >= 0 && len(out) < limit; i-- {
			r := rows[i]
			if !match(r, q) {
				continue
			}
			out = append(out, r)
		}
		return out
	}
	out := make([]Record, 0, limit)
	if q.Kind != "" {
		out = scan(m.rows[q.Kind], out)
	} else {
		for _, k := range AllKinds() {
			out = scan(m.rows[k], out)
			if len(out) >= limit {
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

func match(r Record, q Query) bool {
	if q.Kind != "" && r.Kind != q.Kind {
		return false
	}
	if q.ProjectID != "" && r.ProjectID != q.ProjectID {
		return false
	}
	if q.UserID != "" && r.UserID != q.UserID {
		return false
	}
	if q.StoryID != "" && r.StoryID != q.StoryID {
		return false
	}
	if q.GateName != "" && r.GateName != q.GateName {
		return false
	}
	if q.Tag != "" {
		found := false
		for _, t := range r.Tags {
			if strings.EqualFold(t, q.Tag) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if q.Substring != "" {
		needle := strings.ToLower(q.Substring)
		if !strings.Contains(strings.ToLower(r.Title), needle) &&
			!strings.Contains(strings.ToLower(r.Body), needle) {
			return false
		}
	}
	return true
}

// Delete removes a record by id. Idempotent — deleting an unknown id
// returns nil so the HTTP layer can map DELETE to 204 unconditionally.
func (m *MemoryStore) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, rows := range m.rows {
		for i, r := range rows {
			if r.ID == id {
				m.rows[k] = append(rows[:i], rows[i+1:]...)
				return nil
			}
		}
	}
	return nil
}

// FormatForContext renders a slice of records as a markdown block
// suitable for inlining into an agent's system / project context.
// Keeps each entry compact so 20 memories cost about 1500 tokens.
func FormatForContext(records []Record) string {
	if len(records) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Project memory (sorted newest-first)\n\n")
	for _, r := range records {
		b.WriteString("- **")
		b.WriteString(string(r.Kind))
		b.WriteString("** · ")
		if r.Title != "" {
			b.WriteString(r.Title)
		}
		if len(r.Tags) > 0 {
			b.WriteString(" ` ")
			b.WriteString(strings.Join(r.Tags, " · "))
			b.WriteString("`")
		}
		b.WriteString("\n")
		if body := strings.TrimSpace(r.Body); body != "" {
			b.WriteString("  ")
			b.WriteString(body)
			b.WriteString("\n")
		}
	}
	return b.String()
}

// ensure compile-time interface satisfaction.
var _ Store = (*MemoryStore)(nil)
