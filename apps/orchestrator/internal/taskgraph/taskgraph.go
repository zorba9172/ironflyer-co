// Package taskgraph is the real multi-agent orchestration primitive
// — Layer 3 of the AI Completion Infrastructure blueprint. Most
// competitors do "multi-agent" by chaining LLM calls sequentially.
// That is not orchestration. Real orchestration requires:
//
//   - explicit task graph (DAG) with dependencies
//   - ready-set scheduling (nodes whose deps are done run next)
//   - ownership boundaries (one node = one writer)
//   - per-node confidence scoring
//   - retry budgets
//   - rollback safety
//   - execution lineage you can inspect after the run
//
// This file provides the primitive. It is intentionally agent-
// agnostic: the orchestrator decides which agent owns which node, and
// the graph just answers "which nodes are ready to run" and "what
// happens if this one fails / retries / blocks." The finisher loop
// can keep its current shape until it actually needs the parallelism;
// new flows (Architect → Coder fanned across N stories → Reviewer
// merge) plug in here without reinventing scheduling logic.

package taskgraph

import (
	"errors"
	"sort"
	"sync"
	"time"
)

// Status is the per-node lifecycle. Terminal states are Done /
// Failed / Skipped; everything else is transient.
type Status string

const (
	StatusPending  Status = "pending"
	StatusReady    Status = "ready"
	StatusRunning  Status = "running"
	StatusDone     Status = "done"
	StatusFailed   Status = "failed"
	StatusSkipped  Status = "skipped"
	StatusBlocked  Status = "blocked"
)

// Node is one unit of work. Owner is a free-form label (typically an
// agent role: "architect", "coder.story-7", "critic") so the graph can
// be inspected for ownership conflicts. DependsOn lists the IDs of
// nodes that must reach StatusDone (or StatusSkipped) before this one
// becomes Ready.
type Node struct {
	ID         string
	Owner      string
	Title      string
	DependsOn  []string

	mu         sync.Mutex
	status     Status
	confidence float64 // 0..1, set by Complete()
	attempts   int
	maxRetries int
	lastErr    string
	startedAt  time.Time
	finishedAt time.Time
}

// NewNode constructs a node with the given dependencies + retry budget.
// maxRetries == 0 means no retries (one attempt only).
func NewNode(id, owner, title string, dependsOn []string, maxRetries int) *Node {
	deps := append([]string(nil), dependsOn...)
	return &Node{
		ID:         id,
		Owner:      owner,
		Title:      title,
		DependsOn:  deps,
		status:     StatusPending,
		maxRetries: maxRetries,
	}
}

func (n *Node) Status() Status {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.status
}

func (n *Node) Confidence() float64 {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.confidence
}

func (n *Node) Attempts() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.attempts
}

func (n *Node) LastErr() string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.lastErr
}

// Graph holds a set of nodes plus a derived adjacency. Safe for
// concurrent use — the scheduling caller may interrogate Ready() while
// workers update node status.
type Graph struct {
	mu    sync.Mutex
	nodes map[string]*Node
}

func New() *Graph {
	return &Graph{nodes: map[string]*Node{}}
}

// Add inserts a node. Duplicate IDs return an error rather than
// silently overwriting — the caller probably has a planning bug if
// they're trying to.
func (g *Graph) Add(n *Node) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.nodes[n.ID]; exists {
		return errors.New("taskgraph: duplicate node id " + n.ID)
	}
	g.nodes[n.ID] = n
	return nil
}

// Validate checks for missing dependency targets and cycles. Returns
// the first problem found; callers should call this once after the
// graph is built and refuse to start scheduling on error.
func (g *Graph) Validate() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	for id, n := range g.nodes {
		for _, dep := range n.DependsOn {
			if _, ok := g.nodes[dep]; !ok {
				return errors.New("taskgraph: node " + id + " depends on missing " + dep)
			}
		}
	}
	// Cycle detection via DFS coloring.
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(g.nodes))
	var visit func(string) error
	visit = func(id string) error {
		color[id] = gray
		for _, dep := range g.nodes[id].DependsOn {
			switch color[dep] {
			case gray:
				return errors.New("taskgraph: cycle detected through " + dep)
			case white:
				if err := visit(dep); err != nil {
					return err
				}
			}
		}
		color[id] = black
		return nil
	}
	for id := range g.nodes {
		if color[id] == white {
			if err := visit(id); err != nil {
				return err
			}
		}
	}
	return nil
}

// Ready returns nodes whose status is Pending and all of whose
// dependencies are Done or Skipped. Stable sort by ID for
// deterministic dequeue order during tests + debugging.
func (g *Graph) Ready() []*Node {
	g.mu.Lock()
	defer g.mu.Unlock()
	var out []*Node
	for _, n := range g.nodes {
		if n.Status() != StatusPending {
			continue
		}
		if g.allDepsResolved(n) {
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (g *Graph) allDepsResolved(n *Node) bool {
	for _, dep := range n.DependsOn {
		d := g.nodes[dep]
		if d == nil {
			return false
		}
		st := d.Status()
		if st != StatusDone && st != StatusSkipped {
			return false
		}
	}
	return true
}

// Start marks the node Running and stamps its start time. Idempotent —
// re-starting a running node is allowed and bumps the attempt counter.
func (g *Graph) Start(id string) error {
	n := g.get(id)
	if n == nil {
		return errors.New("taskgraph: no node " + id)
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.status = StatusRunning
	n.attempts++
	if n.startedAt.IsZero() {
		n.startedAt = time.Now().UTC()
	}
	return nil
}

// Complete marks a successful run with a confidence score.
func (g *Graph) Complete(id string, confidence float64) error {
	n := g.get(id)
	if n == nil {
		return errors.New("taskgraph: no node " + id)
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.status = StatusDone
	n.confidence = confidence
	n.finishedAt = time.Now().UTC()
	return nil
}

// Fail marks the node failed. If attempts are still under the retry
// budget the node returns to Pending so Ready() can pick it up again;
// otherwise it lands in StatusFailed permanently.
func (g *Graph) Fail(id string, err error) Status {
	n := g.get(id)
	if n == nil {
		return StatusFailed
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if err != nil {
		n.lastErr = err.Error()
	}
	if n.attempts <= n.maxRetries {
		n.status = StatusPending
		return StatusPending
	}
	n.status = StatusFailed
	n.finishedAt = time.Now().UTC()
	return StatusFailed
}

// Skip marks a node skipped — useful when an upstream decision
// invalidates this work (e.g. spec dropped a story).
func (g *Graph) Skip(id string) error {
	n := g.get(id)
	if n == nil {
		return errors.New("taskgraph: no node " + id)
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.status = StatusSkipped
	n.finishedAt = time.Now().UTC()
	return nil
}

func (g *Graph) get(id string) *Node {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.nodes[id]
}

// Snapshot returns a flat, JSON-friendly view of the graph for
// inspection endpoints. Includes status, timing, confidence, attempts,
// and the dependency edges so a dashboard can render the DAG.
func (g *Graph) Snapshot() []NodeView {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]NodeView, 0, len(g.nodes))
	for _, n := range g.nodes {
		n.mu.Lock()
		out = append(out, NodeView{
			ID:         n.ID,
			Owner:      n.Owner,
			Title:      n.Title,
			DependsOn:  append([]string(nil), n.DependsOn...),
			Status:     n.status,
			Confidence: n.confidence,
			Attempts:   n.attempts,
			MaxRetries: n.maxRetries,
			LastErr:    n.lastErr,
			StartedAt:  n.startedAt,
			FinishedAt: n.finishedAt,
		})
		n.mu.Unlock()
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// NodeView is the read-only DTO Snapshot emits.
type NodeView struct {
	ID         string    `json:"id"`
	Owner      string    `json:"owner"`
	Title      string    `json:"title"`
	DependsOn  []string  `json:"dependsOn"`
	Status     Status    `json:"status"`
	Confidence float64   `json:"confidence"`
	Attempts   int       `json:"attempts"`
	MaxRetries int       `json:"maxRetries"`
	LastErr    string    `json:"lastErr,omitempty"`
	StartedAt  time.Time `json:"startedAt,omitempty"`
	FinishedAt time.Time `json:"finishedAt,omitempty"`
}

// IsDone returns true when every node is in a terminal state. The
// scheduler loops until IsDone returns true OR Ready() returns nothing
// while non-terminal nodes remain (deadlock detection).
func (g *Graph) IsDone() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, n := range g.nodes {
		st := n.Status()
		if st == StatusPending || st == StatusReady || st == StatusRunning || st == StatusBlocked {
			return false
		}
	}
	return true
}

// Deadlocked returns true when there are non-terminal nodes but
// nothing in the ready set — usually a planning bug (a node's
// dependencies failed) the scheduler should surface loudly.
func (g *Graph) Deadlocked() bool {
	if g.IsDone() {
		return false
	}
	return len(g.Ready()) == 0
}
