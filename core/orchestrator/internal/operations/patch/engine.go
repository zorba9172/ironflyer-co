// Package patch is the patch lifecycle engine. AI never mutates files
// directly. Every change is a Patch that goes through validate → preview
// → apply → snapshot → verify → rollback if needed.
package patch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"ironflyer/core/orchestrator/internal/ai/domain"
	"ironflyer/core/orchestrator/internal/operations/metrics"
	"ironflyer/core/orchestrator/internal/business/profitguardctx"
	"ironflyer/core/orchestrator/internal/operations/store"
)

// executionIDFromCtx returns the active execution id stamped onto ctx
// by profitguardctx, or "" when the call is outside an execution
// (CLI / dev / test / smoke). The patch engine threads this through to
// the ArtifactStoreHook so the hook adapter can resolve workload tier
// without re-plumbing the value through every call site.
func executionIDFromCtx(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := profitguardctx.ExecutionID(ctx)
	return id
}

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
	// OpSymbol is a symbol-level AST patch ("in function Foo in bar.go,
	// replace the body with ..."). The change carries a SymbolRef + an
	// Action ("replace_body", "replace_signature", "insert_after",
	// "delete") + NewSource. At Propose-time the engine resolves the
	// symbol via tree-sitter, rewrites the affected byte range, and
	// materialises the result into Content so the rest of the lifecycle
	// (snapshot, runtime apply, gates, rollback) sees it as a normal
	// full-file Update. Symbol metadata is retained on the FileChange
	// for the human review UI.
	//
	// Requires the `treesitter` build tag. The default build refuses
	// symbol patches with a clean error so the Coder can retry with an
	// anchor-patch instead.
	OpSymbol Op = "symbol"
)

// SymbolRef identifies a named declaration inside a source file. The
// engine matches by (Kind, Name) and, when set, Receiver — so a method
// like `func (b *Bar) Baz()` is addressed as
// {Kind:"method", Receiver:"Bar", Name:"Baz"}.
type SymbolRef struct {
	// Kind is one of: "function", "method", "class", "type", "const",
	// "var", "interface", "struct". The set of recognised kinds depends
	// on the grammar; unknown kinds error cleanly at Propose-time.
	Kind string `json:"kind"`
	// Receiver is the receiver-type name for "method" kinds (Go) or the
	// enclosing class name for class-method kinds (TS / Python / Rust).
	// Empty for free functions.
	Receiver string `json:"receiver,omitempty"`
	// Name is the symbol's identifier.
	Name string `json:"name"`
}

// SymbolAction enumerates the AST-level edits the engine knows how to
// perform on a resolved symbol node.
type SymbolAction string

const (
	SymbolReplaceBody      SymbolAction = "replace_body"
	SymbolReplaceSignature SymbolAction = "replace_signature"
	SymbolInsertAfter      SymbolAction = "insert_after"
	SymbolDelete           SymbolAction = "delete"
)

type FileChange struct {
	Op      Op     `json:"op"`
	Path    string `json:"path"`
	Content string `json:"content,omitempty"`
	// Anchor + Replacement are used by OpReplace / OpInsertAfter. Ignored
	// by OpCreate/Update/Delete.
	Anchor      string `json:"anchor,omitempty"`
	Replacement string `json:"replacement,omitempty"`

	// Symbol / SymbolAction / NewSource are used by OpSymbol — the
	// AST-level symbol patch path. The engine resolves Symbol against
	// the file's parse tree, performs SymbolAction with NewSource, and
	// (on success) materialises the rewritten file body into Content
	// and switches Op to OpUpdate so the downstream lifecycle is
	// agnostic to whether the edit was anchor- or AST-derived.
	// Diff is populated by the symbol applier for the review UI.
	Symbol       *SymbolRef   `json:"symbol,omitempty"`
	SymbolAction SymbolAction `json:"symbolAction,omitempty"`
	NewSource    string       `json:"newSource,omitempty"`
	Diff         string       `json:"diff,omitempty"`

	// BaseHash is the sha256-hex of the file body at Propose-time. The
	// engine populates it for any change that targets an existing file
	// (Update / Delete / Replace / InsertAfter / Symbol-derived Update).
	// At Apply-time, if the current body's hash differs, the engine
	// triggers a 3-way merge (see merge.go). Empty for OpCreate.
	BaseHash string `json:"baseHash,omitempty"`

	// BaseBody captures the file body that BaseHash was computed over.
	// Used as the "base" leg of the 3-way merge. Not serialised over
	// the GraphQL wire (the human review UI doesn't need it) but kept
	// in-memory so Apply can do a clean merge without re-fetching.
	BaseBody string `json:"-"`

	// AppliedConflict, when non-empty, indicates Apply detected a
	// concurrent edit and the merger emitted conflict markers. The
	// patch lands in StatusConflicted until a human resolves it.
	AppliedConflict string `json:"appliedConflict,omitempty"`
}

type Status string

const (
	StatusProposed   Status = "proposed"
	StatusValidated  Status = "validated"
	StatusApplied    Status = "applied"
	StatusRejected   Status = "rejected"
	StatusRolled     Status = "rolled-back"
	// StatusConflicted is set by Apply when the 3-way merger detected
	// concurrent edits in one or more target files and emitted
	// conflict markers instead of writing a clean apply. The UI shows
	// a 3-way diff and asks the human to resolve.
	StatusConflicted Status = "conflicted"
)

type Patch struct {
	ID        string         `json:"id"`
	ProjectID string         `json:"projectId"`
	Author    string         `json:"author"`
	Title     string         `json:"title"`
	Summary   string         `json:"summary"`
	Changes   []FileChange   `json:"changes"`
	Issues    []domain.Issue `json:"issues,omitempty"`
	Status    Status         `json:"status"`
	CreatedAt time.Time      `json:"createdAt"`
	AppliedAt *time.Time     `json:"appliedAt,omitempty"`

	// StageID, when non-empty, ties this patch to a PatchStage so the
	// UI can group review across a multi-file logical change. See
	// staging.go.
	StageID string `json:"stageId,omitempty"`

	// Conflicts holds per-file 3-way merge results when Apply landed
	// in StatusConflicted. Keyed by file path.
	Conflicts map[string]PatchConflict `json:"conflicts,omitempty"`
}

// PatchConflict captures the three legs of a 3-way merge for the
// review UI when Apply discovers a concurrent edit. Markers contains
// the merged file body with `<<<<<<<`, `=======`, `>>>>>>>` blocks
// where the merger couldn't pick a winner.
type PatchConflict struct {
	Path    string `json:"path"`
	Base    string `json:"base"`
	Ours    string `json:"ours"`
	Theirs  string `json:"theirs"`
	Markers string `json:"markers"`
}

type Engine struct {
	mu        sync.RWMutex
	projects  store.Store
	patches   map[string]Patch
	order     []string
	snapshots *snapshotStore
	stages    StagingStore

	onProposed   func(p Patch)
	onApplied    func(p Patch)
	onRolledBack func(p Patch, snapshotID string)

	// gateRunner, when set, is invoked after a rollback to re-run the
	// finisher gates (typically Lint / Tests / Security) against the
	// restored tree and surface a "rollback brought the project back
	// to green/red" verdict in the RollbackResult. Wired by the host
	// process so the patch package doesn't import finisher.
	gateRunner GateRunner

	// artifactStoreHook is the V22 ProfitGuard BeforeArtifactStore
	// hook. Apply consults it before writing a patch whose total
	// content size exceeds artifactStoreThreshold; a Stop / KillBranch
	// verdict short-circuits the Apply with ErrPatchBlocked. nil-safe.
	// Wired by the integration agent in cmd/orchestrator/main.go via
	// WithArtifactStoreHook.
	artifactStoreHook       ArtifactStoreHook
	artifactStoreThreshold  int64
}

// ErrPatchBlocked is returned by Apply when an ArtifactStoreHook
// rejects the write. The error surface is intentionally distinct from
// "patch not validated" so the caller's retry policy can tell the
// economic stop apart from a structural failure.
var ErrPatchBlocked = errors.New("patch: blocked by profitguard")

// ArtifactStoreHook is the narrow seam the patch engine uses to
// consult ProfitGuard before persisting a large patch. The action
// string matches profitguard.Action wire values ("continue",
// "stop", "kill_branch", ...); reason is the human-readable cause.
// Implementations MUST be cheap — Apply holds no lock while calling
// the hook, but the hook is on the critical path of every large
// patch.
type ArtifactStoreHook interface {
	BeforeArtifactStore(ctx context.Context, executionID string, sizeBytes int64) (action, reason string, err error)
}

// defaultArtifactStoreThreshold is the byte budget above which the
// hook is consulted. 1 MiB matches the V22 storage workload tier in
// the ProfitGuard margin map.
const defaultArtifactStoreThreshold int64 = 1 << 20

// GateRunner is the narrow contract the patch engine uses to verify
// that a rolled-back tree still passes the same gates the pre-rollback
// tree passed. The host (cmd/orchestrator) wires this to the
// finisher.Engine.RunGate method so the patch package stays free of
// the finisher dependency.
type GateRunner interface {
	RunGate(ctx context.Context, projectID string, gate string) (domain.GateState, error)
}

func NewEngine(projects store.Store) *Engine {
	return &Engine{
		projects:  projects,
		patches:   make(map[string]Patch),
		snapshots: newSnapshotStore(),
		stages:    NewMemoryStagingStore(),
	}
}

// WithStagingStore swaps the in-memory staging store for a persistent
// one (typically a Postgres-backed implementation). Call once at
// startup before serving traffic. Passing nil resets to memory.
func (e *Engine) WithStagingStore(s StagingStore) *Engine {
	if s == nil {
		e.stages = NewMemoryStagingStore()
		return e
	}
	e.stages = s
	return e
}

// WithGateRunner wires the post-rollback verification hook. When set,
// Engine.Rollback will re-run the lint / tests / security gates after
// the project tree is restored and surface the verdicts in the
// RollbackResult. nil disables verification.
func (e *Engine) WithGateRunner(g GateRunner) *Engine {
	e.gateRunner = g
	return e
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

// WithArtifactStoreHook wires the V22 ProfitGuard BeforeArtifactStore
// hook. Apply consults the hook before persisting a patch whose total
// content size exceeds the threshold; Stop / KillBranch verdicts cause
// Apply to return ErrPatchBlocked without mutating the project. Pass
// nil to disable. threshold <=0 keeps the package default (1 MiB) —
// callers pass a real value only when their workload tier needs a
// tighter or looser bar than the storage default.
func (e *Engine) WithArtifactStoreHook(h ArtifactStoreHook, threshold int64) *Engine {
	e.artifactStoreHook = h
	if threshold > 0 {
		e.artifactStoreThreshold = threshold
	} else {
		e.artifactStoreThreshold = defaultArtifactStoreThreshold
	}
	return e
}

// EstimatedBytes returns the total byte count Apply would write for
// this patch. Used by the ArtifactStoreHook gate and by callers that
// want a cheap "is this patch large?" probe without iterating Changes
// twice.
func (p Patch) EstimatedBytes() int64 {
	var n int64
	for _, c := range p.Changes {
		n += int64(len(c.Content)) + int64(len(c.Replacement)) + int64(len(c.NewSource))
	}
	return n
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
	// Symbol-level (AST) patches resolve first: tree-sitter parses the
	// target file, locates the named symbol, rewrites the affected byte
	// range, and materialises the result into Content + flips Op to
	// OpUpdate. After this pass the rest of the pipeline (anchor
	// validation, syntax pre-check, snapshot, apply, runtime write,
	// gates, rollback) is op-uniform — it has no idea the change was
	// AST-derived. Failures here become Issues so the Coder can retry
	// with an anchor-patch.
	symIssues := resolveSymbolPatches(&proj, p.Changes)
	issues := append([]domain.Issue{}, symIssues...)
	issues = append(issues, e.Validate(p)...)
	// Anchor checks are project-aware so they must run here, not in
	// Validate (which is intentionally pure). Verify each OpReplace /
	// OpInsertAfter anchor occurs exactly once in the target file. Less
	// than one → "anchor not found"; more than one → ambiguous (refuse).
	issues = append(issues, validateAnchors(&proj, p.Changes)...)
	// Capture the per-file baseline hash + body so Apply can detect the
	// "user edited the file between propose and apply" race and trigger
	// a 3-way merge. Skipped for OpCreate (no base) and OpDelete (no
	// body to merge).
	captureBaseHashes(&proj, p.Changes)
	// Telemetry — propose-size + per-kind counter.
	for _, c := range p.Changes {
		metrics.ObservePatchKind(string(kindOfChange(c)))
		metrics.ObservePatchSize(len(c.Content) + len(c.Replacement) + len(c.NewSource))
	}
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
		case OpSymbol:
			// Shape-level guard only. The project-aware resolution +
			// AST rewrite happen in resolveSymbolPatches.
			if c.Symbol == nil || c.Symbol.Name == "" {
				issues = append(issues, domain.Issue{Severity: domain.SeverityError, Message: "symbol patch requires symbol.name", Path: c.Path})
			}
			if c.SymbolAction == "" {
				issues = append(issues, domain.Issue{Severity: domain.SeverityError, Message: "symbol patch requires symbolAction", Path: c.Path})
			}
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
	return e.ApplyCtx(context.Background(), id)
}

// ApplyCtx is the ctx-aware variant of Apply. The legacy Apply(id) is
// a thin wrapper that calls ApplyCtx(context.Background(), id) — every
// existing caller keeps working unchanged. ProfitGuard-aware callers
// thread a real context through ApplyCtx so the BeforeArtifactStore
// hook can honour cancellation and decision telemetry.
func (e *Engine) ApplyCtx(ctx context.Context, id string) (Patch, error) {
	e.mu.Lock()
	p, ok := e.patches[id]
	e.mu.Unlock()
	if !ok {
		return Patch{}, errors.New("patch not found")
	}
	if p.Status != StatusValidated {
		return Patch{}, errors.New("patch not validated")
	}

	// V22 BeforeArtifactStore hook. Consulted only for patches whose
	// total content footprint exceeds the storage threshold. The hook
	// adapter wired in main.go owns the executionID lookup (typically
	// from a ctx-bound profitguard.ExecutionID); we pass empty when
	// the context has no execution id so the hook can choose to allow
	// or deny based on size alone.
	if e.artifactStoreHook != nil {
		size := p.EstimatedBytes()
		threshold := e.artifactStoreThreshold
		if threshold <= 0 {
			threshold = defaultArtifactStoreThreshold
		}
		if size > threshold {
			execID := executionIDFromCtx(ctx)
			action, _, hookErr := e.artifactStoreHook.BeforeArtifactStore(ctx, execID, size)
			if hookErr != nil {
				return Patch{}, hookErr
			}
			switch action {
			case "stop", "kill_branch":
				return Patch{}, ErrPatchBlocked
			}
		}
	}

	// Take a pre-apply snapshot so a downstream gate verification failure
	// (or an explicit Engine.Rollback call) can restore the tree to its
	// previous state without depending on the AI to re-author the inverse
	// patch. We snapshot regardless of whether the caller will use it — the
	// per-project ring is bounded so the memory cost is fixed.
	if _, snapErr := e.Snapshot(p.ProjectID, p.ID, "pre-apply: "+p.Title); snapErr != nil {
		return Patch{}, snapErr
	}

	// 3-way merge probe. For every change with a captured BaseHash, check
	// whether the current file body still hashes to the same value. If
	// not, the user (or another agent) edited the file between propose
	// and apply, and we need to merge instead of stomping. Conflicts
	// land on the FileChange's AppliedConflict + the Patch.Conflicts
	// map for the review UI.
	conflicts := map[string]PatchConflict{}
	mergeOutcome := "clean"
	currentProj, getErr := e.projects.Get(p.ProjectID)
	if getErr != nil {
		return Patch{}, getErr
	}
	for i := range p.Changes {
		c := &p.Changes[i]
		if c.BaseHash == "" {
			continue
		}
		curBody, found := lookupFileBody(&currentProj, c.Path)
		if !found {
			continue
		}
		if hashOf(curBody) == c.BaseHash {
			continue
		}
		// Concurrent edit. Compute "theirs" = what this patch would
		// produce against the BASE body, then merge base/ours/theirs.
		theirs := projectedBody(*c)
		merged, conflicted := threeWayMerge(c.BaseBody, curBody, theirs)
		conf := PatchConflict{
			Path:    c.Path,
			Base:    c.BaseBody,
			Ours:    curBody,
			Theirs:  theirs,
			Markers: merged,
		}
		if conflicted {
			conflicts[c.Path] = conf
			c.AppliedConflict = merged
			mergeOutcome = "conflict"
			continue
		}
		// Clean merge — switch the change to a full-file Update with the
		// merged body. The downstream applier sees a normal Update.
		c.Op = OpUpdate
		c.Content = merged
		if mergeOutcome == "clean" {
			mergeOutcome = "merged"
		}
	}
	if len(conflicts) > 0 {
		p.Conflicts = conflicts
		p.Status = StatusConflicted
		e.mu.Lock()
		e.patches[id] = p
		e.mu.Unlock()
		metrics.ObservePatchApplyOutcome("conflict")
		return p, errors.New("patch has merge conflicts — resolve in the UI before re-applying")
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
	metrics.ObservePatchApplyOutcome(mergeOutcome)
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

// PatchSummary is a compact projection of one applied patch, shaped
// for the wow-loop bundle's PatchSource adapter. Only the fields the
// bundle actually consumes ride here — the wow-loop builder does not
// need the full Patch with its Changes / Conflicts / Issues payload.
type PatchSummary struct {
	PatchID       string
	ProjectID     string
	ExecutionID   string
	AffectedPaths []string
	AppliedAt     time.Time
}

// ListByExecution returns every applied patch tagged with the given
// executionID, oldest first. The engine does not currently index
// patches by executionID — Patch carries no ExecutionID field and
// the staging store mirrors that shape — so today this surface
// returns an empty slice and the wow-loop's "what changed" panel
// degrades to "no data yet".
//
// The wow-loop adapter (wireup/wowloop.go) prefers the
// execution.Service.PatchAppliedEventsByExecution path which reads
// patch.applied.v1 events out of the canonical execution_events
// feed; that path lights up the moment finisher.Apply starts
// recording the event.
//
// TODO(wave-3): when the patch lifecycle starts tagging patches
// with the active executionID (either by extending the Patch struct
// or by indexing through a side table), replace this empty
// implementation with a real filter over e.patches.
func (e *Engine) ListByExecution(_ context.Context, executionID string) ([]PatchSummary, error) {
	if e == nil || executionID == "" {
		return nil, nil
	}
	// Returning a non-nil empty slice keeps the contract uniform
	// across backends — callers can range over the result without
	// nil-guards.
	return []PatchSummary{}, nil
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

// captureBaseHashes records the sha256-hex of every target file body at
// Propose-time. Apply uses these to detect concurrent edits between
// propose and apply — the trigger for a 3-way merge.
func captureBaseHashes(proj *domain.Project, changes []FileChange) {
	for i := range changes {
		c := &changes[i]
		if c.Op == OpCreate || c.Op == OpDelete {
			continue
		}
		body, found := lookupFileBody(proj, c.Path)
		if !found {
			continue
		}
		c.BaseBody = body
		c.BaseHash = hashOf(body)
	}
}

// hashOf returns the sha256-hex of body. Cheap, deterministic — used
// as the version-token for the 3-way merge probe.
func hashOf(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

// projectedBody returns what the file body would be if the patch's
// change were applied to its captured BaseBody (no merge). Used as the
// "theirs" leg of the 3-way merge.
func projectedBody(c FileChange) string {
	switch c.Op {
	case OpUpdate, OpCreate:
		return c.Content
	case OpReplace:
		return strings.Replace(c.BaseBody, c.Anchor, c.Replacement, 1)
	case OpInsertAfter:
		return strings.Replace(c.BaseBody, c.Anchor, c.Anchor+c.Replacement, 1)
	case OpDelete:
		return ""
	}
	return c.BaseBody
}

// kindOfChange returns a stable label for the ironflyer_patch_kind_total
// metric so dashboards can plot anchor vs symbol vs whole-file traffic.
func kindOfChange(c FileChange) string {
	switch c.Op {
	case OpSymbol:
		switch c.SymbolAction {
		case SymbolReplaceBody:
			return "symbol_replace_body"
		case SymbolReplaceSignature:
			return "symbol_replace_signature"
		case SymbolInsertAfter:
			return "symbol_insert_after"
		case SymbolDelete:
			return "symbol_delete"
		}
		return "symbol"
	case OpReplace, OpInsertAfter:
		return "anchor"
	case OpCreate:
		return "create"
	case OpDelete:
		return "delete"
	case OpUpdate:
		// A symbol-derived materialised Update keeps its Symbol field set.
		if c.Symbol != nil {
			return "symbol_materialised"
		}
		return "update"
	}
	return "unknown"
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
