// Package finisher — parallel critic. The classic Coder → Critic chain
// runs the Critic only after the Coder has committed its patch. By then
// the cheap signal arrives late: the user has already watched a doomed
// generation stream by and we've already paid the Coder's full token
// bill. This file teaches the engine to run the Critic *alongside* the
// Coder, sliding a window over the live token stream and firing
// "critique-in-progress" provider rounds every few seconds. Live
// concerns surface as `critic_partial` SSE events so the dashboard
// chip lights up in real time; a `severity: blocker` finding cancels
// the Coder mid-stream and writes a clear audit entry.
//
// Switch-off is supported: set IRONFLYER_CRITIC_PARALLEL=false to keep
// the original Run() shape. Operators who run a lot of quick gates may
// prefer to skip the extra provider spend.

package finisher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"ironflyer/core/orchestrator/internal/ai/agents"
	"ironflyer/core/orchestrator/internal/operations/audit"
	"ironflyer/core/orchestrator/internal/ai/domain"
	"ironflyer/core/orchestrator/internal/operations/metrics"
	"ironflyer/core/orchestrator/internal/ai/providers"
)

// criticParallelEnv is the env var that toggles the in-progress critic
// loop. Default-on matches the product promise (concerns surface live);
// operators set it to "false" to fall back to the original sequential
// path for cheap gates where the extra provider spend isn't justified.
const criticParallelEnv = "IRONFLYER_CRITIC_PARALLEL"

// Tuning constants. Kept here rather than threaded through the public
// API because the parallel-critic surface is internal and operators
// tune via env (the only knob exposed today is the on/off flag).
const (
	// criticWindowBytes caps the rolling-window snapshot we hand to
	// the critic per tick. The window must be small enough to keep
	// per-tick provider cost bounded but large enough to carry a
	// meaningful slice of fresh output. 4 KiB is roughly two full
	// React components or one medium Go function.
	criticWindowBytes = 4 * 1024

	// criticTickInterval is the cadence of "critique what we have so
	// far" rounds while the Coder is still streaming.
	criticTickInterval = 5 * time.Second

	// criticMaxConcurrent caps the number of partial critiques that
	// may be in flight at once. The tick fires every `criticTickInterval`
	// regardless of how slow the previous critique was; without a cap
	// a slow provider could pile up dozens of inflight rounds.
	criticMaxConcurrent = 3

	// criticPartialPromptCap caps prompt growth — the spec + window
	// shouldn't trip prompt-token limits on the critic provider even
	// for very long stories.
	criticPartialPromptCap = 12 * 1024
)

// partialCriticOutput is the structured JSON the live critic emits per
// tick. We keep findings tiny so the critic provider can be a cheap
// model — quantity isn't the point, the live blocker is.
type partialCriticOutput struct {
	Findings []partialCriticFinding `json:"findings"`
}

type partialCriticFinding struct {
	Concern  string `json:"concern"`
	Severity string `json:"severity"` // "info" | "warning" | "blocker"
	Snippet  string `json:"snippet"`
}

const partialCriticInstruction = `You are observing a code generation in PROGRESS. You are NOT reviewing a finished patch — the assistant is still writing. You see the user story and the last few KB of generated text only.

Reply with EXACTLY one JSON object:
{
  "findings": [
    { "concern": "...", "severity": "info" | "warning" | "blocker", "snippet": "..." }
  ]
}

Rules:
- Maximum 2 findings. Pick the most consequential.
- "blocker" is reserved for irrecoverable issues: wrong stack, broken syntax in a finished file, secret leak, spec-violating shape. A "blocker" causes the generation to be aborted immediately.
- "warning" surfaces a concern the assistant should address before finishing.
- "info" is for observations that don't require action.
- If nothing is wrong yet, return {"findings":[]}.
- No prose, no markdown, no commentary outside the JSON.`

// criticParallelEnabled reads the env toggle. Empty / missing / any
// non-"false" value enables the live critic; explicit "false" /
// "0" / "off" turns it off.
func criticParallelEnabled() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(criticParallelEnv)))
	switch v {
	case "false", "0", "off", "no":
		return false
	default:
		return true
	}
}

// runCoderStreamingWithCritic is the streaming-first replacement for
// registry.Run when the parallel critic is enabled. It mirrors the
// Coder side of registry.Run (text + thinking accumulation, DeltaDone
// usage capture) and concurrently feeds the token stream through a
// sliding window to a critic goroutine that fires partial critique
// rounds. The returned Result matches registry.Run's contract so
// callers can swap one for the other without changing JSON parsing.
//
// When the partial critic surfaces a blocker, it cancels the Coder
// context — the streaming provider unwinds quickly and we return an
// "aborted by parallel critic" error so the surrounding retry loop
// fails this attempt fast.
//
// abortedErr is the sentinel returned when the critic killed the
// Coder mid-stream. Callers that want to distinguish a critic abort
// from a provider error can check errors.Is.
var abortedByCriticErr = errors.New("coder aborted by parallel critic")

func (e *Engine) runCoderStreamingWithCritic(
	ctx context.Context,
	projectID string,
	story domain.UserStory,
	task agents.Task,
) (agents.Result, error) {
	// Toggle: when disabled, route to the existing single-shot Run so
	// the sequential path stays available with zero overhead.
	if !criticParallelEnabled() {
		return e.registry.Run(ctx, task)
	}
	// If the Critic agent isn't registered, there's nothing to run in
	// parallel — fall back to the standard path so the Coder still
	// gets dispatched normally and the post-patch runCritic is a no-op.
	if _, ok := e.registry.Get(agents.RoleCritic); !ok {
		return e.registry.Run(ctx, task)
	}

	// Per-call cancellable context so the critic goroutine can abort
	// the Coder mid-stream. We DON'T derive abortCtx from a separate
	// background — it must inherit deadlines, bearer, and workspace
	// values from the caller's ctx.
	streamCtx, cancelStream := context.WithCancel(ctx)
	defer cancelStream()

	deltaCh, err := e.registry.RunStream(streamCtx, task)
	if err != nil {
		return agents.Result{}, err
	}

	win := newCriticWindow(criticWindowBytes)

	// abortReason carries the human description set by the critic
	// goroutine when it triggers cancelStream. The streaming loop
	// reads it after the stream drains so the returned error message
	// is specific.
	var (
		abortMu      sync.Mutex
		abortReason  string
		bytesAtAbort int64
		abortFlag    atomic.Bool
	)
	markAbort := func(reason string, byteCount int64) {
		abortMu.Lock()
		if abortReason == "" {
			abortReason = reason
			bytesAtAbort = byteCount
		}
		abortMu.Unlock()
		abortFlag.Store(true)
		cancelStream()
	}

	// criticDone closes when the critic goroutine exits cleanly. We
	// wait on it before returning so partial critiques in flight
	// finish (or unwind on cancelStream) before the caller sees the
	// Result — leak-free even on the abort path.
	criticDone := make(chan struct{})
	go e.runPartialCriticLoop(streamCtx, projectID, story, task, win, markAbort, criticDone)

	// Drain the Coder stream. Mirrors registry.Run's Coder branch but
	// without tool-use handling — the Coder role is configured with
	// CapTools downstream, however tool round-trips are non-streaming
	// from the caller's perspective (registry.Run handles them
	// internally). For the parallel-critic shape we stick to the
	// straight text path: tool_use deltas are uncommon on the JSON
	// patch reply and are accumulated as empty text if they slip
	// through. We log nothing in the hot path.
	var (
		textBuf     strings.Builder
		thinkingBuf strings.Builder
		provider    string
		tokens      int
		cost        float64
		bytesSeen   int64
		streamErr   error
	)

	for d := range deltaCh {
		switch d.Type {
		case providers.DeltaText:
			if d.Text != "" {
				textBuf.WriteString(d.Text)
				bytesSeen += int64(len(d.Text))
				win.append(d.Text)
			}
		case providers.DeltaThinking:
			if d.Text != "" {
				thinkingBuf.WriteString(d.Text)
				// Thinking tokens also feed the window — they often
				// reveal a wrong-track decision before the Coder
				// writes the first patched byte.
				win.append(d.Text)
				bytesSeen += int64(len(d.Text))
			}
		case providers.DeltaDone:
			provider = d.Provider
			if d.Usage != nil {
				tokens += d.Usage.InputTokens + d.Usage.OutputTokens
				cost += d.Usage.CostUSD
			}
		case providers.DeltaError:
			streamErr = d.Err
		}
	}

	// Signal end-of-stream to the critic loop and wait for it to drain
	// before we return so we never leak the goroutine across runs.
	win.close()
	<-criticDone

	// Abort path: critic killed the stream. Emit a structured Coder
	// event and an audit entry that explicitly attributes the abort.
	if abortFlag.Load() {
		metrics.ObserveCriticBlockerAborted()
		abortMu.Lock()
		reason := abortReason
		offset := bytesAtAbort
		abortMu.Unlock()
		msg := fmt.Sprintf("Coder aborted by parallel critic at byte offset %d: %s", offset, reason)
		e.emit(projectID, domain.Event{
			ID: newEventID(), Step: StepCoder, Agent: string(agents.RoleCoder),
			Status:    StatusFailed,
			Message:   fmtErr(ErrCodePatchInvalid, msg),
			CreatedAt: time.Now().UTC(),
		})
		// Audit trail: a blocker abort is exactly the kind of action a
		// production-trust operator needs to replay later. We reuse
		// the existing AgentDispatch action so the audit vocabulary
		// stays small (per CLAUDE.md "list short on purpose").
		if proj, err := e.projects.Get(projectID); err == nil {
			e.recordAudit(streamCtx, audit.Entry{
				Action:    audit.ActionAgentDispatch,
				Outcome:   audit.OutcomeBlocked,
				UserID:    proj.OwnerID,
				ProjectID: projectID,
				StoryID:   story.ID,
				AgentRole: string(agents.RoleCritic),
				Summary:   msg,
				Attrs: map[string]any{
					"abortedRole": string(agents.RoleCoder),
					"byteOffset":  offset,
					"reason":      reason,
				},
			})
		}
		return agents.Result{
			Role:     task.Role,
			Output:   textBuf.String(),
			Thinking: thinkingBuf.String(),
			Provider: provider,
			Tokens:   tokens,
			CostUSD:  cost,
		}, fmt.Errorf("%w: %s", abortedByCriticErr, reason)
	}

	if streamErr != nil {
		return agents.Result{}, streamErr
	}

	return agents.Result{
		Role:     task.Role,
		Output:   textBuf.String(),
		Thinking: thinkingBuf.String(),
		Provider: provider,
		Tokens:   tokens,
		CostUSD:  cost,
	}, nil
}

// criticWindow is the sliding window of recent Coder output the
// partial critic samples each tick. Implemented as a mutex-guarded
// ring buffer so a slow critic goroutine can't tear a half-written
// snapshot, and so the streaming hot path takes the lock only for
// the duration of an append.
type criticWindow struct {
	mu       sync.Mutex
	buf      []byte // ring; len(buf) <= cap(buf) == capacity
	capacity int
	closed   bool
	// totalBytes is the count of bytes ever written to the window.
	// Used so the abort audit entry can name the (approximate) token
	// offset of the trigger event.
	totalBytes int64
}

func newCriticWindow(capacity int) *criticWindow {
	if capacity <= 0 {
		capacity = criticWindowBytes
	}
	return &criticWindow{
		buf:      make([]byte, 0, capacity),
		capacity: capacity,
	}
}

func (w *criticWindow) append(s string) {
	if s == "" {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.totalBytes += int64(len(s))
	// Fast path: room for the whole chunk.
	if len(w.buf)+len(s) <= w.capacity {
		w.buf = append(w.buf, s...)
		return
	}
	// Slow path: chunk overflows the window. Keep only the most
	// recent w.capacity bytes of the (existing tail + chunk).
	if len(s) >= w.capacity {
		// New chunk alone exceeds capacity — discard everything
		// older and keep the tail.
		w.buf = append(w.buf[:0], s[len(s)-w.capacity:]...)
		return
	}
	// Drop just enough from the front of buf to make room.
	keep := w.capacity - len(s)
	if keep < len(w.buf) {
		w.buf = append(w.buf[:0], w.buf[len(w.buf)-keep:]...)
	}
	w.buf = append(w.buf, s...)
}

// snapshot returns a copy of the window's current contents plus the
// running byte count. Copy isolates the critic goroutine from
// subsequent appends; it's cheap (capacity is bounded) and worth the
// safety.
func (w *criticWindow) snapshot() (string, int64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.buf) == 0 {
		return "", w.totalBytes
	}
	out := make([]byte, len(w.buf))
	copy(out, w.buf)
	return string(out), w.totalBytes
}

func (w *criticWindow) close() {
	w.mu.Lock()
	w.closed = true
	w.mu.Unlock()
}

// runPartialCriticLoop is the goroutine that fires partial critiques
// every `criticTickInterval` while the Coder is streaming. It exits
// when the streamCtx is cancelled OR when the window is closed by
// the streaming loop. inFlight caps the number of concurrent
// critiques so a slow provider can't pile rounds up indefinitely.
func (e *Engine) runPartialCriticLoop(
	streamCtx context.Context,
	projectID string,
	story domain.UserStory,
	coderTask agents.Task,
	win *criticWindow,
	markAbort func(reason string, byteCount int64),
	done chan<- struct{},
) {
	defer close(done)

	tick := time.NewTicker(criticTickInterval)
	defer tick.Stop()

	sem := make(chan struct{}, criticMaxConcurrent)
	var wg sync.WaitGroup

	// runOne dispatches a single partial-critique provider round.
	// Returns true when the round emitted a blocker — caller uses
	// the return only for early-exit accounting; the markAbort
	// callback has already fired by the time we return.
	runOne := func(snapshot string, atBytes int64) {
		defer wg.Done()
		defer func() { <-sem }()

		// Build the critic task. We DO NOT inject MCP tool catalogues
		// — partial critic answers JSON, not tool calls.
		criticTask := agents.Task{
			Role:        agents.RoleCritic,
			Project:     coderTask.Project,
			Goal:        buildPartialCriticGoal(story, snapshot),
			UserBearer:  coderTask.UserBearer,
			WorkspaceID: coderTask.WorkspaceID,
		}

		started := time.Now()
		res, err := e.registry.Run(streamCtx, criticTask)
		metrics.ObserveCriticPartialLatency(time.Since(started))
		if err != nil {
			// Fail-open: a partial-critic provider error must NEVER
			// stop the Coder. Cost is already metered through the
			// BillingGuard at the router layer; we drop the round.
			return
		}

		var out partialCriticOutput
		if err := unmarshalJSONFromText(res.Output, &out); err != nil {
			return
		}
		if len(out.Findings) == 0 {
			return
		}

		for _, f := range out.Findings {
			payload, _ := json.Marshal(map[string]any{
				"concern":  f.Concern,
				"severity": f.Severity,
				"snippet":  truncateForEvent(f.Snippet, 240),
				"atBytes":  atBytes,
			})
			e.emit(projectID, domain.Event{
				ID: newEventID(), Step: StepCriticPartial, Agent: string(agents.RoleCritic),
				Status:    StatusRunning,
				Message:   "critic_partial " + string(payload),
				CreatedAt: time.Now().UTC(),
			})
			metrics.ObserveCriticPartialEmitted()

			if strings.EqualFold(strings.TrimSpace(f.Severity), "blocker") {
				reason := truncateForEvent(f.Concern, 240)
				if reason == "" {
					reason = "blocker without concern text"
				}
				markAbort(reason, atBytes)
				return
			}
		}
	}

	for {
		select {
		case <-streamCtx.Done():
			// Drain in-flight critiques before signalling done so the
			// caller's wait on the done channel sees no leaks. The
			// goroutines themselves observe streamCtx cancellation
			// through registry.Run / router.CompleteStream.
			wg.Wait()
			return
		case <-tick.C:
			snap, atBytes := win.snapshot()
			if strings.TrimSpace(snap) == "" {
				continue
			}
			// Concurrency cap: try to acquire a slot without blocking.
			// If the cap is full we skip this tick — the next one
			// will pick up a fresher window anyway.
			select {
			case sem <- struct{}{}:
			default:
				continue
			}
			wg.Add(1)
			go runOne(snap, atBytes)
		}
	}
}

// buildPartialCriticGoal renders the prompt the partial critic sees
// each tick. We re-include the story on every call so the critic
// answers a self-contained question — caching the partial output
// across ticks is intentionally NOT done (each round must inspect
// the latest window).
func buildPartialCriticGoal(story domain.UserStory, window string) string {
	var b strings.Builder
	b.WriteString("Live critic on a Coder generation that is STILL streaming for story ")
	b.WriteString(story.ID)
	b.WriteString(".\n\n# Story\n")
	b.WriteString("As ")
	b.WriteString(story.As)
	b.WriteString("\nI want ")
	b.WriteString(story.IWant)
	if story.SoThat != "" {
		b.WriteString("\nSo that ")
		b.WriteString(story.SoThat)
	}
	if len(story.Acceptance) > 0 {
		b.WriteString("\nAcceptance:\n")
		for _, a := range story.Acceptance {
			b.WriteString("  - ")
			b.WriteString(a)
			b.WriteString("\n")
		}
	}
	b.WriteString("\n# Partial generation so far (last bytes only)\n")
	// Cap the window again inside the prompt — the per-tick prompt
	// budget compounds across runs and the critic provider may have
	// a tighter context window than the Coder.
	if len(window) > criticWindowBytes {
		window = window[len(window)-criticWindowBytes:]
	}
	if total := b.Len() + len(window) + len(partialCriticInstruction); total > criticPartialPromptCap {
		// Trim the window further to fit. Worst-case the critic sees
		// only the most-recent 1 KiB — still meaningful for blocker
		// detection (syntax breaks, wrong stack).
		excess := total - criticPartialPromptCap
		if excess < len(window) {
			window = window[excess:]
		} else {
			window = ""
		}
	}
	b.WriteString(window)
	b.WriteString("\n")
	b.WriteString(partialCriticInstruction)
	return b.String()
}

// truncateForEvent keeps SSE event payloads bounded so a chatty
// critic provider can't blow up the Redis publish path.
func truncateForEvent(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
