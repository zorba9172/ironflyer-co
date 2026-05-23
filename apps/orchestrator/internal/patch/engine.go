// Package patch is the patch lifecycle engine. AI never mutates files
// directly. Every change is a Patch that goes through validate → preview
// → apply → snapshot → verify → rollback if needed.
package patch

import (
	"errors"
	"strings"
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
	// OpReplace is an anchor-based partial-file rewrite. The agent supplies
	// an Anchor (a unique substring that already exists in the file) plus a
	// Replacement that takes the anchor's place. The engine validates that
	// the anchor occurs EXACTLY once before applying — anything else is a
	// rejection, no line-number guessing. Compared to a unified diff this
	// is robust to whitespace/line drift while still trimming the output
	// budget by an order of magnitude on large files.
	OpReplace Op = "replace"
	// OpInsertAfter inserts new lines immediately after a unique anchor.
	// Useful for adding routes, imports, exports without rewriting the
	// whole file. Same uniqueness invariant as OpReplace.
	OpInsertAfter Op = "insert_after"
)

type FileChange struct {
	Op      Op     `json:"op"`
	Path    string `json:"path"`
	Content string `json:"content,omitempty"`
	// Anchor + Replacement are used by OpReplace / OpInsertAfter. Ignored
	// by OpCreate/Update/Delete.
	Anchor      string `json:"anchor,omitempty"`
	Replacement string `json:"replacement,omitempty"`
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

	onProposed   func(p Patch)
	onApplied    func(p Patch)
	onRolledBack func(p Patch, snapshotID string)
}

func NewEngine(projects store.Store) *Engine {
	return &Engine{
		projects:  projects,
		patches:   make(map[string]Patch),
		snapshots: newSnapshotStore(),
	}
}

// WithOnProposed registers a callback invoked AFTER a patch is
// successfully proposed (status == proposed). nil disables it.
func (e *Engine) WithOnProposed(fn func(p Patch)) *Engine {
	e.onProposed = fn
	return e
}

// WithOnApplied registers a callback invoked AFTER a patch reaches
// status == applied AND its files have been written into the project
// store. Snapshot of the prior state has been captured by the time
// this fires, so the callback can reference the rollback id.
func (e *Engine) WithOnApplied(fn func(p Patch)) *Engine {
	e.onApplied = fn
	return e
}

// WithOnRolledBack registers a callback invoked AFTER a patch is
// rolled back from `applied` state, with the snapshot id that was
// used to restore the project. nil disables it.
func (e *Engine) WithOnRolledBack(fn func(p Patch, snapshotID string)) *Engine {
	e.onRolledBack = fn
	return e
}

func (e *Engine) Propose(p Patch) (Patch, error) {
	if p.ProjectID == "" {
		return Patch{}, errors.New("projectId required")
	}
	proj, err := e.projects.Get(p.ProjectID)
	if err != nil {
		return Patch{}, err
	}
	if p.ID == "" {
		p.ID = newID("patch")
	}
	p.Status = StatusProposed
	p.CreatedAt = time.Now().UTC()
	issues := e.Validate(p)
	// Anchor checks are project-aware so they must run here, not in
	// Validate (which is intentionally pure). Verify each OpReplace /
	// OpInsertAfter anchor occurs exactly once in the target file. Less
	// than one → "anchor not found"; more than one → ambiguous (refuse).
	issues = append(issues, validateAnchors(&proj, p.Changes)...)
	if len(issues) > 0 {
		p.Issues = issues
		p.Status = StatusRejected
	} else {
		p.Status = StatusValidated
	}
	e.mu.Lock()
	e.patches[p.ID] = p
	e.order = append(e.order, p.ID)
	stored := e.patches[p.ID]
	cb := e.onProposed
	e.mu.Unlock()
	if cb != nil {
		cb(stored)
	}
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
		case OpReplace, OpInsertAfter:
			if c.Anchor == "" {
				issues = append(issues, domain.Issue{Severity: domain.SeverityError, Message: "anchor required for " + string(c.Op), Path: c.Path})
			}
			// Replacement may legitimately be empty for OpReplace ("delete
			// this block") so we don't require it; the file existence /
			// uniqueness check happens at apply time when we hold the
			// current body.
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
	applied := e.patches[id]
	cb := e.onApplied
	e.mu.Unlock()
	if cb != nil {
		cb(applied)
	}
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

// validateAnchors enforces the OpReplace / OpInsertAfter contract: the file
// must exist and the anchor must occur in it exactly once. We reject "found
// 0" (the AI misremembered the source) AND "found N>1" (the substitution
// would be ambiguous). The full-file ops (OpCreate/Update/Delete) are
// ignored here — they are validated by the generic Validate pass.
func validateAnchors(proj *domain.Project, changes []FileChange) []domain.Issue {
	var issues []domain.Issue
	for _, c := range changes {
		if c.Op != OpReplace && c.Op != OpInsertAfter {
			continue
		}
		var body string
		var found bool
		for _, f := range proj.Files {
			if f.Path == c.Path {
				body = f.Content
				found = true
				break
			}
		}
		if !found {
			issues = append(issues, domain.Issue{
				Severity: domain.SeverityError,
				Message:  "anchor target file does not exist",
				Path:     c.Path,
				Hint:     "either OpCreate the file first or use the actual path",
			})
			continue
		}
		count := strings.Count(body, c.Anchor)
		switch {
		case count == 0:
			issues = append(issues, domain.Issue{
				Severity: domain.SeverityError,
				Message:  "anchor not found in file",
				Path:     c.Path,
				Hint:     "the anchor must be a verbatim substring of the current file body",
			})
		case count > 1:
			issues = append(issues, domain.Issue{
				Severity: domain.SeverityError,
				Message:  "anchor matches more than once — would be ambiguous",
				Path:     c.Path,
				Hint:     "extend the anchor with surrounding context until it is unique",
			})
		}
	}
	return issues
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
	case OpReplace, OpInsertAfter:
		// Anchor-based partial rewrite. Find the file, locate the anchor,
		// substitute. The Validate / pre-apply gate already guaranteed the
		// anchor exists exactly once; this is a pure string substitution.
		for i := range p.Files {
			if p.Files[i].Path != c.Path {
				continue
			}
			old := p.Files[i].Content
			var updated string
			if c.Op == OpReplace {
				updated = strings.Replace(old, c.Anchor, c.Replacement, 1)
			} else { // OpInsertAfter
				updated = strings.Replace(old, c.Anchor, c.Anchor+c.Replacement, 1)
			}
			p.Files[i].Content = updated
			p.Files[i].Size = len(updated)
			return
		}
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
