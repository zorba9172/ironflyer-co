// Package patch is the patch lifecycle engine. AI never mutates files
// directly. Every change is a Patch that goes through validate → preview
// → apply → snapshot → verify → rollback if needed.
package patch

import (
	"errors"
	"sync"
	"time"

	"ironflyer/apps/orchestrator/internal/domain"
	"ironflyer/apps/orchestrator/internal/store"
)

type Op string

const (
	OpCreate Op = "create"
	OpUpdate Op = "update"
	OpDelete Op = "delete"
)

type FileChange struct {
	Op      Op     `json:"op"`
	Path    string `json:"path"`
	Content string `json:"content,omitempty"`
}

type Status string

const (
	StatusProposed  Status = "proposed"
	StatusValidated Status = "validated"
	StatusApplied   Status = "applied"
	StatusRejected  Status = "rejected"
	StatusRolled    Status = "rolled-back"
)

type Patch struct {
	ID         string        `json:"id"`
	ProjectID  string        `json:"projectId"`
	Author     string        `json:"author"`
	Title      string        `json:"title"`
	Summary    string        `json:"summary"`
	Changes    []FileChange  `json:"changes"`
	Issues     []domain.Issue `json:"issues,omitempty"`
	Status     Status        `json:"status"`
	CreatedAt  time.Time     `json:"createdAt"`
	AppliedAt  *time.Time    `json:"appliedAt,omitempty"`
}

type Engine struct {
	mu        sync.RWMutex
	projects  store.Store
	patches   map[string]Patch
	order     []string
	snapshots *snapshotStore
}

func NewEngine(projects store.Store) *Engine {
	return &Engine{
		projects:  projects,
		patches:   make(map[string]Patch),
		snapshots: newSnapshotStore(),
	}
}

func (e *Engine) Propose(p Patch) (Patch, error) {
	if p.ProjectID == "" {
		return Patch{}, errors.New("projectId required")
	}
	if _, err := e.projects.Get(p.ProjectID); err != nil {
		return Patch{}, err
	}
	if p.ID == "" {
		p.ID = newID("patch")
	}
	p.Status = StatusProposed
	p.CreatedAt = time.Now().UTC()
	if issues := e.Validate(p); len(issues) > 0 {
		p.Issues = issues
		p.Status = StatusRejected
	} else {
		p.Status = StatusValidated
	}
	e.mu.Lock()
	e.patches[p.ID] = p
	e.order = append(e.order, p.ID)
	e.mu.Unlock()
	return p, nil
}

// Validate enforces scope, basic syntax sanity, and forbidden paths.
func (e *Engine) Validate(p Patch) []domain.Issue {
	var issues []domain.Issue
	for _, c := range p.Changes {
		if c.Path == "" {
			issues = append(issues, domain.Issue{Severity: domain.SeverityError, Message: "empty path in change"})
			continue
		}
		if containsAny(c.Path, "..", "/etc/", "/root/", ".ssh/") {
			issues = append(issues, domain.Issue{Severity: domain.SeverityCritical, Message: "forbidden path", Path: c.Path})
		}
		switch c.Op {
		case OpCreate, OpUpdate:
			if c.Content == "" {
				issues = append(issues, domain.Issue{Severity: domain.SeverityWarning, Message: "empty content", Path: c.Path})
			}
		case OpDelete:
			// nothing
		default:
			issues = append(issues, domain.Issue{Severity: domain.SeverityError, Message: "unknown op: " + string(c.Op), Path: c.Path})
		}
	}
	// Deterministic syntax pre-check on the proposed file bodies. Runs in
	// milliseconds and rejects obvious LLM hallucinations (broken Go,
	// malformed JSON / YAML, unbalanced delimiters in TS/JS/Python/etc.)
	// before they ever land on disk or trigger a Reviewer round-trip.
	issues = append(issues, syntaxIssues(p.Changes)...)
	return issues
}

func (e *Engine) Apply(id string) (Patch, error) {
	e.mu.Lock()
	p, ok := e.patches[id]
	e.mu.Unlock()
	if !ok {
		return Patch{}, errors.New("patch not found")
	}
	if p.Status != StatusValidated {
		return Patch{}, errors.New("patch not validated")
	}

	// Take a pre-apply snapshot so a downstream gate verification failure
	// (or an explicit Engine.Rollback call) can restore the tree to its
	// previous state without depending on the AI to re-author the inverse
	// patch. We snapshot regardless of whether the caller will use it — the
	// per-project ring is bounded so the memory cost is fixed.
	if _, snapErr := e.Snapshot(p.ProjectID, p.ID, "pre-apply: "+p.Title); snapErr != nil {
		return Patch{}, snapErr
	}

	_, err := e.projects.Update(p.ProjectID, func(proj *domain.Project) {
		for _, c := range p.Changes {
			applyChange(proj, c)
		}
		proj.Events = append(proj.Events, domain.Event{
			ID:        newID("evt"),
			Step:      "patch",
			Message:   "patch applied: " + p.Title,
			Status:    "done",
			CreatedAt: time.Now().UTC(),
		})
	})
	if err != nil {
		return Patch{}, err
	}

	now := time.Now().UTC()
	p.Status = StatusApplied
	p.AppliedAt = &now
	e.mu.Lock()
	e.patches[id] = p
	e.mu.Unlock()
	return p, nil
}

func (e *Engine) List(projectID string) []Patch {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]Patch, 0, len(e.order))
	for _, id := range e.order {
		p := e.patches[id]
		if projectID == "" || p.ProjectID == projectID {
			out = append(out, p)
		}
	}
	return out
}

func (e *Engine) Get(id string) (Patch, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	p, ok := e.patches[id]
	if !ok {
		return Patch{}, errors.New("patch not found")
	}
	return p, nil
}

func applyChange(p *domain.Project, c FileChange) {
	switch c.Op {
	case OpDelete:
		filtered := p.Files[:0]
		for _, f := range p.Files {
			if f.Path != c.Path {
				filtered = append(filtered, f)
			}
		}
		p.Files = filtered
	case OpCreate, OpUpdate:
		for i := range p.Files {
			if p.Files[i].Path == c.Path {
				p.Files[i].Content = c.Content
				p.Files[i].Size = len(c.Content)
				return
			}
		}
		p.Files = append(p.Files, domain.FileNode{
			Path: c.Path, Type: "file", Content: c.Content, Size: len(c.Content),
		})
	}
}

func containsAny(s string, parts ...string) bool {
	for _, p := range parts {
		if indexOf(s, p) >= 0 {
			return true
		}
	}
	return false
}

func indexOf(s, sub string) int {
	n, m := len(s), len(sub)
	if m == 0 || m > n {
		return -1
	}
	for i := 0; i+m <= n; i++ {
		if s[i:i+m] == sub {
			return i
		}
	}
	return -1
}

var idCounter int
var idMu sync.Mutex

func newID(prefix string) string {
	idMu.Lock()
	defer idMu.Unlock()
	idCounter++
	return prefix + "-" + time.Now().UTC().Format("20060102150405") + "-" + itoa(idCounter)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
