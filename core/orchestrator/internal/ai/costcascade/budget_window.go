package costcascade

// budget_windows.go — Multi-window Agent Budget Manager.
//
// This file adds session-, daily-, and task-scoped budgets on TOP of the
// two money laws Ironflyer already enforces, WITHOUT duplicating or
// replacing them. To be unambiguous about what this layer is and is not,
// here is the existing financial spine it composes beside (read these
// before changing anything here):
//
//   - business/wallet/wallet.go — the prepaid credit wallet. Hold /
//     Release / Debit move REAL money against a tenant's
//     BalanceUSD/HoldUSD. Hard law 1: "no execution starts without
//     budget."
//   - business/ledger/ledger.go — the append-only money trail. Every
//     reservation, provider charge, refund and release is an immutable
//     ledger.Entry. This is the audit/proof surface for "did this run at
//     positive gross margin?" (law 3).
//   - business/budget/{plan,enforcer}.go — the per-MONTH plan cap. The
//     budget.Enforcer.Admit gate blocks/downgrades a call against
//     Plan.CostCapUSD using ledger.SpentByUser over the calendar month.
//   - business/execution/{types,cost}.go — per-EXECUTION attribution
//     (ProviderCostUSD/SandboxCostUSD/… on the executions row, broken out
//     by execution.CostKind).
//
// What is MISSING from that spine, and what this file supplies:
//
//   The spine accounts money over two horizons — the calendar MONTH
//   (plan cap) and the single EXECUTION (wallet hold). An autonomous
//   agent loop has finer-grained economic boundaries that neither covers:
//
//     * a SESSION — one interactive operator sitting / one agent run that
//       may span many model calls but should not silently burn unbounded
//       credit;
//     * a DAILY budget — a rolling UTC-day ceiling that auto-rolls at
//       midnight, independent of the monthly plan reset;
//     * a TASK budget — one logical unit of work ("add auth", "fix the
//       failing build") that should carry its own ceiling so a runaway
//       sub-goal cannot eat the whole session.
//
//   And the spine attributes cost per-user / per-execution / per
//   CostKind, but NOT "cost per FEATURE" or "cost per TASK" — the
//   observability the V22 vision asks for ("name what is unclosed
//   end-to-end" / cost mirrors of AI state). This file maintains those
//   rollups too.
//
// COMPOSITION CONTRACT (this is a soft, advisory pre-gate — NOT the law):
//
//   The real money still moves through wallet (Hold/Debit/Release) and is
//   recorded in the ledger. WindowedBudgetManager is an IN-MEMORY,
//   advisory ceiling consulted BEFORE the wallet/plan gates and a
//   bookkeeping sink consulted AFTER the provider charge materialises. A
//   caller's correct ordering is:
//
//     1. WindowedBudgetManager.Admit(...)   // advisory window ceilings
//     2. budget.Enforcer.Admit(...)         // hard monthly plan cap
//     3. wallet.Hold(...)                   // hard law 1 (real funds)
//     4. … run the model call …
//     5. wallet.Debit(...) + ledger.Charge  // real money + audit row
//     6. WindowedBudgetManager.Charge(...)  // mirror the actual spend here
//
//   Steps 2/3/5 are the source of truth. If this manager and the wallet
//   ever disagree, the WALLET WINS — this layer never moves a cent and is
//   safe to wipe/restart. Every method degrades to a permissive no-op on
//   any internal inconsistency so it can NEVER block a correct, funded
//   execution: a missing window admits, a math error admits, a nil
//   manager admits. It tightens spend; it never fabricates a denial that
//   would break a legitimately-funded run.
//
// All money is github.com/shopspring/decimal, matching the spine.

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
)

// BudgetWindowKind is the horizon a BudgetKey is scoped to. The three
// kinds are independent ceilings: a single Admit/Charge is keyed to
// exactly one window, and a caller that wants "must fit session AND daily
// AND task" simply calls Admit three times (or uses AdmitAll). Keeping the
// kinds orthogonal mirrors how the spine keeps month (plan) and execution
// (wallet hold) orthogonal rather than nesting them.
type BudgetWindowKind string

const (
	// BudgetWindowSession scopes spend to one interactive operator
	// sitting / one agent run. Reset is EXPLICIT (BudgetResetWindow or
	// BudgetEndSession) — a session ends when the caller says it ends,
	// never on a clock.
	BudgetWindowSession BudgetWindowKind = "session"

	// BudgetWindowDaily scopes spend to a rolling UTC day. It AUTO-ROLLS
	// at UTC midnight: the first Admit/Charge/Remaining observed on a new
	// UTC day zeroes the spent counter for that window. This is
	// deliberately UTC to match budget.PeriodResetAt / ledger monthStart,
	// which are also UTC.
	BudgetWindowDaily BudgetWindowKind = "daily"

	// BudgetWindowTask scopes spend to one logical unit of work. Reset is
	// EXPLICIT, like a session — a task window is retired with
	// BudgetResetWindow when the task closes.
	BudgetWindowTask BudgetWindowKind = "task"
)

// budgetValidKind reports whether k is one of the three known horizons.
// Unknown kinds are treated as "no ceiling configured" → admit.
func budgetValidKind(k BudgetWindowKind) bool {
	switch k {
	case BudgetWindowSession, BudgetWindowDaily, BudgetWindowTask:
		return true
	default:
		return false
	}
}

// BudgetKey identifies one budget window. Kind selects the horizon; ID is
// the opaque scope identifier within that horizon (a session id, a tenant
// id for the daily ceiling, a task id). Feature/Task are OPTIONAL labels
// carried through to the per-feature / per-task rollups so the same Charge
// that decrements a window also attributes cost for observability — they
// do NOT affect which counter is debited (that is (Kind, ID) only).
//
// Example keys:
//
//	{Kind: BudgetWindowSession, ID: "sess_abc", Feature: "auth", Task: "wire-oauth"}
//	{Kind: BudgetWindowDaily,   ID: "tenant_42"}
//	{Kind: BudgetWindowTask,    ID: "task_login", Feature: "auth"}
type BudgetKey struct {
	Kind    BudgetWindowKind
	ID      string
	Feature string
	Task    string
}

// budgetWindowID is the internal map key for a window's counter. Feature
// and Task are intentionally excluded so all spend under the same
// (Kind, ID) accumulates into one ceiling regardless of label.
type budgetWindowID struct {
	Kind BudgetWindowKind
	ID   string
}

func (k BudgetKey) windowID() budgetWindowID {
	return budgetWindowID{Kind: k.Kind, ID: k.ID}
}

// BudgetDecision is the advisory verdict of an Admit call. It mirrors the
// shape of budget.Decision (Admit + RemainingUSD + Reason) so a caller can
// treat a window verdict and a plan verdict uniformly. Admit==true is the
// default-safe outcome; a false is only ever produced when a configured
// ceiling would be exceeded by the estimate.
type BudgetDecision struct {
	// Admit is the advisory verdict. true = the estimate fits the
	// remaining window allowance (or no ceiling is configured). false =
	// charging estUSD would push the window over its cap.
	Admit bool

	// RemainingUSD is the window's remaining allowance AFTER accounting
	// for current spend but BEFORE the proposed estUSD is charged. It is
	// decimal.Zero (never negative) when the window is exhausted, and the
	// configured cap when no spend has landed yet. When no cap is
	// configured for the window it reports decimal.Zero with Admit=true —
	// callers distinguish "unlimited" via Reason.
	RemainingUSD decimal.Decimal

	// Reason is a short human/telemetry string explaining the verdict
	// ("ok", "no window ceiling configured", "session budget exhausted",
	// "would overshoot daily budget"). Never an error sentinel — this
	// layer is advisory and propagates nothing fatal.
	Reason string

	// Kind / ID echo the window the decision was made against, so a
	// caller logging a denial knows which horizon tripped without
	// re-deriving it.
	Kind BudgetWindowKind
	ID   string
}

// budgetWindow is the in-memory counter for one (Kind, ID). cap is the
// ceiling (decimal.Zero = no ceiling → always admit); spent is the running
// total Charge has accumulated; dayAnchor is the UTC day the spent counter
// currently belongs to (used only by daily windows for the midnight roll).
type budgetWindow struct {
	cap       decimal.Decimal
	spent     decimal.Decimal
	dayAnchor time.Time // UTC midnight of the day `spent` covers (daily only)
}

// BudgetWindowOptions configures a WindowedBudgetManager. The zero value
// is usable: it yields a manager with no default caps (every window admits
// until SetCap is called), a real-clock UTC day source, and a disabled
// logger. All fields are optional.
type BudgetWindowOptions struct {
	// DefaultSessionCapUSD / DefaultDailyCapUSD / DefaultTaskCapUSD are
	// the caps applied to a window the first time it is touched if no
	// explicit SetCap has been recorded for it. decimal.Zero (the zero
	// value) means "no default ceiling" — that horizon admits everything
	// until a cap is set. These mirror, in spirit, budget.Plan.CostCapUSD
	// but at finer horizons; they are NOT derived from the plan and never
	// override it.
	DefaultSessionCapUSD decimal.Decimal
	DefaultDailyCapUSD   decimal.Decimal
	DefaultTaskCapUSD    decimal.Decimal

	// Now is an injectable clock returning the current time. nil →
	// time.Now. Used so the daily-window midnight roll is deterministic
	// for callers that want to drive it; this layer holds no other global
	// state. The manager always normalises to UTC internally regardless
	// of the zone Now returns.
	Now func() time.Time

	// Logger receives advisory denials at debug level. A zero
	// zerolog.Logger (disabled) is fine — this layer never logs at a
	// level that would be noisy in production.
	Logger zerolog.Logger
}

// WindowedBudgetManager holds the in-memory session/daily/task ceilings
// and the per-feature / per-task cost rollups. It is fully thread-safe:
// every public method takes the single mutex. It owns NO real money — see
// the composition contract at the top of this file.
//
// Lifetime: construct one per orchestrator process (it is process-global
// advisory state, not per-request) and consult it on the hot path. It is
// cheap to wipe and restart because the wallet/ledger remain the source of
// truth.
type WindowedBudgetManager struct {
	mu sync.Mutex

	windows map[budgetWindowID]*budgetWindow

	// caps holds explicit per-window overrides set via SetCap. Looked up
	// before the per-kind default so a caller can pin one specific
	// session/task to a tighter or looser ceiling than the default.
	caps map[budgetWindowID]decimal.Decimal

	// Per-feature / per-task cost rollups — the "cost per feature / cost
	// per task" observability the vision asks for. These accumulate the
	// ACTUAL charged spend (Charge), never the estimates (Admit), so they
	// match what the ledger booked. They are append-only counters; a
	// reset of a window does not clear them (a feature's lifetime cost
	// spans many sessions/tasks).
	byFeature map[string]decimal.Decimal
	byTask    map[string]decimal.Decimal

	defSession decimal.Decimal
	defDaily   decimal.Decimal
	defTask    decimal.Decimal

	now    func() time.Time
	logger zerolog.Logger
}

// NewWindowedBudgetManager builds a manager from opts. It never returns an
// error — every option degrades to a safe default — so wiring it is always
// safe. A nil clock falls back to time.Now.
func NewWindowedBudgetManager(opts BudgetWindowOptions) *WindowedBudgetManager {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &WindowedBudgetManager{
		windows:    make(map[budgetWindowID]*budgetWindow),
		caps:       make(map[budgetWindowID]decimal.Decimal),
		byFeature:  make(map[string]decimal.Decimal),
		byTask:     make(map[string]decimal.Decimal),
		defSession: nonNegOrZero(opts.DefaultSessionCapUSD),
		defDaily:   nonNegOrZero(opts.DefaultDailyCapUSD),
		defTask:    nonNegOrZero(opts.DefaultTaskCapUSD),
		now:        now,
		logger:     opts.Logger,
	}
}

// nonNegOrZero clamps a cap to >= 0. A negative configured cap is
// nonsensical (it would deny everything); we treat it as "no ceiling" so a
// mis-configuration can never wedge a funded execution shut.
func nonNegOrZero(d decimal.Decimal) decimal.Decimal {
	if d.IsNegative() {
		return decimal.Zero
	}
	return d
}

// budgetUTCDay returns UTC midnight of the day t falls in — the anchor a
// daily window's spent counter is bound to. Matches the UTC-midnight
// convention of ledger.monthStart / budget.PeriodResetAt.
func budgetUTCDay(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

// SetCap pins an explicit ceiling for one window, overriding the per-kind
// default. A cap of decimal.Zero (or negative) clears the override back to
// "use the kind default". Caller-facing convenience for "this particular
// task gets a tighter budget than the global task default". Thread-safe.
func (m *WindowedBudgetManager) SetCap(key BudgetKey, capUSD decimal.Decimal) {
	if m == nil || !budgetValidKind(key.Kind) {
		return
	}
	id := key.windowID()
	m.mu.Lock()
	defer m.mu.Unlock()
	if capUSD.IsPositive() {
		m.caps[id] = capUSD
	} else {
		delete(m.caps, id)
	}
	// If the window already exists, refresh its cap so a mid-flight
	// re-cap takes effect on the next Admit without waiting for a reset.
	if w, ok := m.windows[id]; ok {
		w.cap = m.capForLocked(id, key.Kind)
	}
}

// capForLocked resolves the effective cap for a window: the explicit
// override if set, else the per-kind default, else decimal.Zero ("no
// ceiling"). Caller holds m.mu.
func (m *WindowedBudgetManager) capForLocked(id budgetWindowID, kind BudgetWindowKind) decimal.Decimal {
	if c, ok := m.caps[id]; ok && c.IsPositive() {
		return c
	}
	switch kind {
	case BudgetWindowSession:
		return m.defSession
	case BudgetWindowDaily:
		return m.defDaily
	case BudgetWindowTask:
		return m.defTask
	default:
		return decimal.Zero
	}
}

// windowLocked fetches (creating if absent) the counter for key, applying
// the daily midnight roll on access. Caller holds m.mu. Returns nil only
// for an unknown kind (which the public methods treat as "admit").
func (m *WindowedBudgetManager) windowLocked(key BudgetKey) *budgetWindow {
	if !budgetValidKind(key.Kind) {
		return nil
	}
	id := key.windowID()
	w, ok := m.windows[id]
	if !ok {
		w = &budgetWindow{
			cap:       m.capForLocked(id, key.Kind),
			spent:     decimal.Zero,
			dayAnchor: budgetUTCDay(m.now()),
		}
		m.windows[id] = w
		return w
	}
	// Daily auto-roll: if we have crossed into a new UTC day since the
	// counter was last anchored, zero the spent total and re-anchor. This
	// is the ONLY automatic reset; session/task windows never roll on a
	// clock.
	if key.Kind == BudgetWindowDaily {
		today := budgetUTCDay(m.now())
		if today.After(w.dayAnchor) {
			w.spent = decimal.Zero
			w.dayAnchor = today
		}
	}
	// Keep the cap fresh in case defaults/overrides changed since
	// creation (cheap and keeps the window authoritative).
	w.cap = m.capForLocked(id, key.Kind)
	return w
}

// remainingLocked returns max(0, cap-spent) for a window, or decimal.Zero
// when the window has no ceiling (cap == 0). Caller holds m.mu. The
// "no ceiling" and "exhausted" cases both report zero remaining; callers
// distinguish them via the cap (which Remaining/Admit expose through
// Reason). Returning zero rather than a sentinel keeps the type pure
// decimal.Decimal as required.
func (w *budgetWindow) remainingLocked() decimal.Decimal {
	if !w.cap.IsPositive() {
		return decimal.Zero
	}
	rem := w.cap.Sub(w.spent)
	if rem.IsNegative() {
		return decimal.Zero
	}
	return rem
}

// Admit is the advisory pre-gate: it asks "would charging estUSD against
// this window stay under its ceiling?" WITHOUT mutating any counter (it is
// a read; the spend is recorded later via Charge once the real cost
// materialises). See the composition contract — this runs BEFORE
// budget.Enforcer.Admit and wallet.Hold, and never substitutes for them.
//
// Degrades to Admit=true on every soft failure: nil manager, unknown
// window kind, non-positive estimate, or no configured ceiling. It only
// returns Admit=false when a real, positive ceiling would be overshot.
func (m *WindowedBudgetManager) Admit(ctx context.Context, key BudgetKey, estUSD decimal.Decimal) BudgetDecision {
	dec := BudgetDecision{Admit: true, RemainingUSD: decimal.Zero, Reason: "ok", Kind: key.Kind, ID: key.ID}
	if m == nil {
		dec.Reason = "no window manager"
		return dec
	}
	if !budgetValidKind(key.Kind) {
		dec.Reason = "unknown window kind — admit"
		return dec
	}

	m.mu.Lock()
	w := m.windowLocked(key)
	if w == nil {
		m.mu.Unlock()
		dec.Reason = "unknown window kind — admit"
		return dec
	}
	if !w.cap.IsPositive() {
		m.mu.Unlock()
		dec.Reason = "no window ceiling configured — admit"
		return dec
	}
	remaining := w.remainingLocked()
	ceiling := w.cap
	spent := w.spent
	m.mu.Unlock()

	dec.RemainingUSD = remaining

	// A non-positive estimate cannot push us over — admit and report the
	// live remaining. (Negative estimates are nonsensical; treat as zero.)
	est := estUSD
	if est.IsNegative() {
		est = decimal.Zero
	}

	if spent.Add(est).GreaterThan(ceiling) {
		dec.Admit = false
		switch {
		case remaining.IsZero():
			dec.Reason = string(key.Kind) + " budget exhausted"
		default:
			dec.Reason = "would overshoot " + string(key.Kind) + " budget"
		}
		m.logger.Debug().
			Str("window_kind", string(key.Kind)).
			Str("window_id", key.ID).
			Str("est_usd", est.String()).
			Str("remaining_usd", remaining.String()).
			Str("cap_usd", ceiling.String()).
			Msg("costcascade: window budget would be overshot (advisory)")
	}
	return dec
}

// AdmitAll is a convenience that ANDs Admit across several keys (e.g.
// "must fit session AND daily AND task"). It returns the FIRST denying
// decision, or the session/last decision when all admit. It performs no
// mutation. Useful so a caller does not have to thread three separate
// Admit calls and short-circuit by hand.
func (m *WindowedBudgetManager) AdmitAll(ctx context.Context, estUSD decimal.Decimal, keys ...BudgetKey) BudgetDecision {
	last := BudgetDecision{Admit: true, Reason: "ok"}
	for _, k := range keys {
		d := m.Admit(ctx, k, estUSD)
		if !d.Admit {
			return d
		}
		last = d
	}
	return last
}

// Charge records ACTUAL spend against the window and the per-feature /
// per-task rollups. Call this AFTER the wallet Debit + ledger.Charge have
// booked the real money (step 6 of the composition contract) so this
// mirror stays consistent with the source of truth. feature/task override
// the labels on the key when non-empty (so a caller can attribute a charge
// to a feature/task discovered only after the call ran).
//
// Degrades to a no-op on nil manager, unknown kind, or non-positive
// amount. Never returns an error: a bookkeeping mirror must never be able
// to fail a real, already-charged execution.
func (m *WindowedBudgetManager) Charge(ctx context.Context, key BudgetKey, actualUSD decimal.Decimal, feature, task string) {
	if m == nil || !actualUSD.IsPositive() {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if budgetValidKind(key.Kind) {
		if w := m.windowLocked(key); w != nil {
			w.spent = w.spent.Add(actualUSD)
		}
	}

	// Attribute to the per-feature / per-task rollups. An explicit
	// feature/task argument wins over the label on the key; both are
	// optional and an empty label simply means "unattributed", which we
	// skip so the rollup maps stay clean.
	f := feature
	if f == "" {
		f = key.Feature
	}
	if f != "" {
		m.byFeature[f] = m.byFeature[f].Add(actualUSD)
	}
	t := task
	if t == "" {
		t = key.Task
	}
	if t != "" {
		m.byTask[t] = m.byTask[t].Add(actualUSD)
	}
}

// Remaining returns the window's remaining allowance (max(0, cap-spent)),
// applying the daily midnight roll on access. Reports decimal.Zero for an
// unknown kind or a window with no configured ceiling — callers wanting to
// distinguish "unlimited" from "exhausted" should use Admit (which sets a
// Reason). Thread-safe.
func (m *WindowedBudgetManager) Remaining(key BudgetKey) decimal.Decimal {
	if m == nil || !budgetValidKind(key.Kind) {
		return decimal.Zero
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	w := m.windowLocked(key)
	if w == nil {
		return decimal.Zero
	}
	return w.remainingLocked()
}

// Spent returns the amount charged against the window so far (post daily
// roll). Companion to Remaining for surfaces that want both the numerator
// and denominator of a window gauge. Thread-safe.
func (m *WindowedBudgetManager) Spent(key BudgetKey) decimal.Decimal {
	if m == nil || !budgetValidKind(key.Kind) {
		return decimal.Zero
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	w := m.windowLocked(key)
	if w == nil {
		return decimal.Zero
	}
	return w.spent
}

// ResetWindow zeroes the spent counter for one window, keeping its cap.
// This is the EXPLICIT reset for session/task windows ("session ended",
// "task closed"); daily windows reset themselves at UTC midnight but may
// also be reset early through here. Does NOT clear the per-feature /
// per-task rollups — those are lifetime cost counters that intentionally
// outlive any single window. Thread-safe; no-op on unknown kind.
func (m *WindowedBudgetManager) ResetWindow(key BudgetKey) {
	if m == nil || !budgetValidKind(key.Kind) {
		return
	}
	id := key.windowID()
	m.mu.Lock()
	defer m.mu.Unlock()
	if w, ok := m.windows[id]; ok {
		w.spent = decimal.Zero
		w.dayAnchor = budgetUTCDay(m.now())
	}
}

// EndSession is the explicit teardown for a session (or task) window: it
// drops the counter entirely so its memory is reclaimed. Equivalent to
// ResetWindow for accounting purposes but also forgets the cap override.
// Use it when a session/task id will never be seen again. Thread-safe.
func (m *WindowedBudgetManager) EndSession(key BudgetKey) {
	if m == nil || !budgetValidKind(key.Kind) {
		return
	}
	id := key.windowID()
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.windows, id)
	delete(m.caps, id)
}

// BudgetSnapshot is the read-only observability projection returned by
// Snapshot. It carries the per-feature and per-task cost rollups (the
// "cost per feature / cost per task" the V22 viz-first vision asks for) and
// a count of live windows. The maps are defensive COPIES — a caller may
// read/sort them without holding the manager's lock and without racing
// concurrent Charges.
type BudgetSnapshot struct {
	// ByFeature maps feature label → lifetime actual cost (USD) charged
	// under it. Mirrors what the ledger booked, broken out by feature so a
	// dashboard can answer "how much did 'auth' cost end-to-end?".
	ByFeature map[string]decimal.Decimal

	// ByTask maps task label → lifetime actual cost (USD) charged under
	// it. Same provenance as ByFeature, at task granularity.
	ByTask map[string]decimal.Decimal

	// LiveWindows is the number of session/daily/task windows currently
	// holding a counter — a cheap health/ops number ("how many open
	// budget scopes is this process tracking?").
	LiveWindows int

	// At is the manager clock reading when the snapshot was taken (UTC).
	At time.Time
}

// Snapshot returns the current per-feature / per-task rollups and live
// window count for an observability surface. Maps are copied so the
// snapshot is safe to retain and iterate after the lock is released.
// Thread-safe; never returns nil maps.
func (m *WindowedBudgetManager) Snapshot() BudgetSnapshot {
	snap := BudgetSnapshot{
		ByFeature: map[string]decimal.Decimal{},
		ByTask:    map[string]decimal.Decimal{},
	}
	if m == nil {
		snap.At = time.Now().UTC()
		return snap
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for f, v := range m.byFeature {
		snap.ByFeature[f] = v
	}
	for t, v := range m.byTask {
		snap.ByTask[t] = v
	}
	snap.LiveWindows = len(m.windows)
	snap.At = m.now().UTC()
	return snap
}
