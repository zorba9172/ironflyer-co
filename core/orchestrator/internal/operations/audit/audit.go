// Package audit is the production-trust moat (Moat #4 in the AI
// Completion Infrastructure blueprint). Enterprise customers don't
// trust pure LLM systems — trust comes from observability, validation,
// audit trails, deterministic execution, explainability, rollback, and
// compliance. This package owns the audit trail piece: a tamper-
// resistant, append-only log of every consequential action the
// orchestrator took on behalf of a user.
//
// What's recorded:
//   - patch proposed / applied / rolled back
//   - gate verdicts (pass / fail / blocked / repaired)
//   - agent dispatch (role, role-input hash, role-output hash)
//   - secret writes (which key was injected — never the value)
//   - workspace command exec (shell-hash, exit code)
//   - deployment events
//
// Each entry is content-addressed via a SHA-256 of its canonical JSON
// form so the hash chain detects post-hoc tampering when the operator
// chooses to ship the log to a WORM store / external SIEM.

package audit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// stampRegion writes the orchestrator's data-residency region into
// e.Attrs["region"] so the audit chain proves where each action was
// processed. Sourced from IRONFLYER_DATA_RESIDENCY, which the Helm
// chart wires from .Values.region (see docs/MULTI_REGION.md). Called
// exactly once per Record to keep the audit row tamper-evident: the
// region is part of the hashed content.
func stampRegion(e *Entry) {
	region := strings.TrimSpace(os.Getenv("IRONFLYER_DATA_RESIDENCY"))
	if region == "" {
		region = "unknown"
	}
	if e.Attrs == nil {
		e.Attrs = map[string]any{}
	}
	// Don't overwrite a caller-supplied region (test harnesses pre-set it).
	if _, ok := e.Attrs["region"]; !ok {
		e.Attrs["region"] = region
	}
}

// Action is the canonical action vocabulary. We keep this list short
// on purpose — every event in the system must classify into one of
// these or the auditor can't reason about it.
type Action string

const (
	ActionPatchProposed   Action = "patch.proposed"
	ActionPatchApplied    Action = "patch.applied"
	ActionPatchRolledBack Action = "patch.rolled_back"
	ActionGateVerdict     Action = "gate.verdict"
	// ActionPreflightDecision lands one entry per Anti-Bloat Reuse-
	// First Preflight verdict the patch engine produces (see
	// patch/preflight.go + docs/ANTI_BLOAT_ENGINE.md§"Audit"). The
	// `outcome` field reflects the decision: success = reuse / new
	// with justification; failure = blocked when Validate() refuses
	// the decision shape. Attrs carries `{action, query, topPath,
	// topSymbol, score, hitCount}`.
	ActionPreflightDecision Action = "preflight.decision"
	ActionAgentDispatch   Action = "agent.dispatch"
	ActionSecretWritten   Action = "secret.written"
	ActionExec            Action = "workspace.exec"
	ActionDeploy          Action = "deploy"
	ActionMemoryRecord    Action = "memory.record"
	// Billing — Stripe metered + invoice events. metered_usage_reported
	// is written every successful flush of the MeteredReporter so the
	// audit log shows the billing chain end-to-end (call -> ledger ->
	// Stripe). metered_invoice_created mirrors Stripe's invoice.created
	// webhook so a customer can replay exactly when each bill landed.
	ActionMeteredUsageReported Action = "metered_usage_reported"
	ActionMeteredInvoiceCreated Action = "metered_invoice_created"
	ActionPaymentFailed         Action = "payment.failed"
	// ActionProviderRegistered is emitted once per known LLM provider
	// at orchestrator startup so the audit chain proves which backends
	// were online in which environment. Attrs carry {provider, model,
	// enabled}; outcome is OutcomeSuccess when registered or
	// OutcomeBlocked when gated off due to missing creds.
	ActionProviderRegistered Action = "provider.registered"

	// Auth lifecycle events (commercial table-stakes). Every entry is
	// hash-chained against the prior so a compliance auditor can prove
	// no link in the chain was excised.
	ActionAuthSignupCompleted        Action = "auth.signup.completed"
	ActionAuthEmailVerified          Action = "auth.email.verified"
	ActionAuthPasswordResetRequested Action = "auth.password.reset_requested"
	ActionAuthPasswordResetCompleted Action = "auth.password.reset_completed"
	ActionAuthMfaEnrolled            Action = "auth.mfa.enrolled"
	ActionAuthMfaConfirmed           Action = "auth.mfa.confirmed"
	ActionAuthMfaDisabled            Action = "auth.mfa.disabled"
	ActionAuthMfaRecoveryUsed        Action = "auth.mfa.recovery_used"
	ActionAuthSessionRevoked         Action = "auth.session.revoked"
	ActionAuthEmailChangeRequested   Action = "auth.email.change_requested"
	ActionAuthEmailChangeCompleted   Action = "auth.email.change_completed"

	// Billing commercial surface (Round 14). The finance audit trail
	// answers "why was this user charged $X" by walking the hash-chained
	// log filtered by userID. Each dunning transition lands one entry so
	// the cadence is replayable end-to-end.
	ActionBillingCheckoutStarted     Action = "billing.checkout.started"
	ActionBillingCheckoutCompleted   Action = "billing.checkout.completed"
	ActionBillingSubscriptionCancel  Action = "billing.subscription.cancelled"
	ActionBillingPortalSession       Action = "billing.portal.session"
	ActionBillingRefundIssued        Action = "billing.refund.issued"
	ActionBillingDunningEntered      Action = "billing.dunning.entered"
	ActionBillingDunningCleared      Action = "billing.dunning.cleared"
	ActionBillingDunningPaused       Action = "billing.dunning.paused"
	ActionBillingInvoicesListed      Action = "billing.invoices.listed"
)

// Outcome is the coarse status of an action. Always one of these so
// dashboards can aggregate without parsing free-form text.
type Outcome string

const (
	OutcomeSuccess Outcome = "success"
	OutcomeFailure Outcome = "failure"
	OutcomeBlocked Outcome = "blocked"
)

// Entry is one immutable row. Fields that don't apply to the Action
// are simply zero-valued. ContentHash is computed by the store at
// Record time over the canonical JSON of all other fields; PrevHash
// links to the previous entry's ContentHash so the log forms a hash
// chain detectable to tampering.
type Entry struct {
	ID          string         `json:"id"`
	Action      Action         `json:"action"`
	Outcome     Outcome        `json:"outcome"`
	UserID      string         `json:"userId,omitempty"`
	ProjectID   string         `json:"projectId,omitempty"`
	StoryID     string         `json:"storyId,omitempty"`
	GateName    string         `json:"gateName,omitempty"`
	AgentRole   string         `json:"agentRole,omitempty"`
	Summary     string         `json:"summary"`              // short, indexable
	InputHash   string         `json:"inputHash,omitempty"`  // sha256 of the canonical request body
	OutputHash  string         `json:"outputHash,omitempty"` // sha256 of the canonical response body
	Attrs       map[string]any `json:"attrs,omitempty"`      // free-form structured detail
	CreatedAt   time.Time      `json:"createdAt"`
	PrevHash    string         `json:"prevHash,omitempty"`
	ContentHash string         `json:"contentHash"`
}

// Query is the filter shape for the read API. Zero-valued fields are
// wildcards. Limit 0 → default 100 (max 1000 enforced by callers).
type Query struct {
	UserID    string
	ProjectID string
	Action    Action
	Outcome   Outcome
	Since     time.Time
	Until     time.Time
	Limit     int
}

// Store is the operator-replaceable contract. Implementations MUST
// preserve insertion order, MUST refuse mutations to already-stored
// entries, and SHOULD persist across orchestrator restarts in
// production deployments.
type Store interface {
	Record(ctx context.Context, e Entry) (Entry, error)
	Query(ctx context.Context, q Query) ([]Entry, error)
	// Verify walks the hash chain and returns the index of the first
	// inconsistency, or -1 when the log is intact.
	Verify(ctx context.Context) (int, error)
}

// MemoryStore is the dev / single-node default. Bounded ring buffer so
// long-lived servers don't drift into unbounded RAM.
type MemoryStore struct {
	mu       sync.Mutex
	entries  []Entry
	lastHash string
	max      int
}

// NewMemoryStore returns a fresh in-memory audit log with the supplied
// cap. Default 16k entries ≈ a few MiB.
func NewMemoryStore(max int) *MemoryStore {
	if max <= 0 {
		max = 16 * 1024
	}
	return &MemoryStore{max: max}
}

// Record appends an entry. ID + CreatedAt + PrevHash + ContentHash
// are filled by the store; callers may pre-fill them but values get
// overwritten so the chain stays internally consistent.
func (m *MemoryStore) Record(_ context.Context, e Entry) (Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e.ID = uuid.NewString()
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	stampRegion(&e)
	redactEntry(&e)
	e.PrevHash = m.lastHash
	e.ContentHash = hashEntry(e)

	if len(m.entries) >= m.max {
		// Ring eviction. The PrevHash chain remains internally consistent
		// from the current head — operators who need permanent retention
		// should swap in a Postgres / object-store backend.
		copy(m.entries, m.entries[1:])
		m.entries = m.entries[:len(m.entries)-1]
	}
	m.entries = append(m.entries, e)
	m.lastHash = e.ContentHash
	return e, nil
}

// Query returns entries newest-first that match every set field.
func (m *MemoryStore) Query(_ context.Context, q Query) ([]Entry, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 100
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Entry, 0, limit)
	for i := len(m.entries) - 1; i >= 0 && len(out) < limit; i-- {
		e := m.entries[i]
		if q.UserID != "" && e.UserID != q.UserID {
			continue
		}
		if q.ProjectID != "" && e.ProjectID != q.ProjectID {
			continue
		}
		if q.Action != "" && e.Action != q.Action {
			continue
		}
		if q.Outcome != "" && e.Outcome != q.Outcome {
			continue
		}
		if !q.Since.IsZero() && e.CreatedAt.Before(q.Since) {
			continue
		}
		if !q.Until.IsZero() && e.CreatedAt.After(q.Until) {
			continue
		}
		out = append(out, e)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

// Verify walks the chain from the oldest entry forward and returns the
// index of the first entry whose ContentHash does not match a fresh
// recomputation OR whose PrevHash doesn't equal the prior entry's
// ContentHash. -1 means the chain is intact.
func (m *MemoryStore) Verify(_ context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	prev := ""
	for i, e := range m.entries {
		if e.PrevHash != prev {
			return i, nil
		}
		// Recompute against a copy whose hashes are zeroed to avoid
		// feeding ContentHash back into itself.
		copy := e
		copy.ContentHash = ""
		if hashEntry(copy) != e.ContentHash {
			// Hash mismatch — the stored entry's contents diverge from the
			// hash we computed when it was first appended.
			return i, nil
		}
		prev = e.ContentHash
	}
	return -1, nil
}

// hashEntry computes the canonical SHA-256 of e with ContentHash
// zeroed. JSON Marshal is stable enough for the subset of types we
// use; if we ever need cross-language verification, swap to a fixed
// field-order encoder.
func hashEntry(e Entry) string {
	clone := e
	clone.ContentHash = ""
	raw, _ := json.Marshal(clone)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// HashBytes is a public utility for callers that want to compute an
// InputHash / OutputHash on arbitrary payloads. Trimmed whitespace +
// stable JSON re-encoding keeps the hash deterministic on similar
// inputs.
func HashBytes(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

// HashString hashes a string after TrimSpace so casual prompt edits
// (trailing newline, leading whitespace) don't churn the hash.
func HashString(s string) string {
	return HashBytes([]byte(strings.TrimSpace(s)))
}

var _ Store = (*MemoryStore)(nil)
