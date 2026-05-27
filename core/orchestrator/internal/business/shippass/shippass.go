// Package shippass implements the outcome-based SKU layer.
//
// A Ship Pass is a fixed-price promise that an Ironflyer project will
// clear every gate in a declared scope before a deadline. The user
// purchases a pass against their wallet at a known price; the wallet
// holds (not debits) the funds while the project is in-flight. When
// every gate in the scope reports a passing verdict the Settler
// debits the held amount, flips the pass to Shipped, and emits an
// OutcomeEvent. If the deadline elapses without a complete pass the
// Settler releases the hold back to the wallet and flips the pass to
// Refunded — the user pays nothing for an unshipped run.
//
// Why this design.
//
//   - The wallet already supports Hold / Release / DebitWithKey with
//     Temporal-safe idempotency (wallet.IdempotentService). Ship Pass
//     reuses that surface verbatim so settlement survives retries.
//
//   - Tier scopes are static: pricing decisions stay in one file
//     (tiers.go) and never leak into resolvers. New tiers ship by
//     editing one slice — never by touching the lifecycle code.
//
//   - The Settler hooks the existing finisher gate verdict stream
//     instead of inventing a new event bus. The orchestrator's gate
//     verdict emission already exists (internal/ai/finisher/events.go);
//     wireup feeds those verdicts here as `Observe` calls.
//
// Hard contracts:
//   - All money flows through wallet.IdempotentService; never bypass it.
//   - Owner check on every read and write: the tenant on a pass MUST
//     match the caller's tenant.
//   - Every lifecycle transition publishes a learning.OutcomeEvent so
//     the Feedback Brain can mine "passes that almost shipped".
package shippass

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"

	"ironflyer/core/orchestrator/internal/ai/domain"
)

// Status is the lifecycle state of a Ship Pass row.
type Status string

const (
	// StatusActive is the in-flight state: funds are held in the
	// wallet, the deadline has not elapsed, not every required gate
	// has reported a passing verdict yet.
	StatusActive Status = "active"
	// StatusShipped is the terminal success state: every gate in the
	// scope passed before the deadline; held funds have been debited
	// and the user is on the hook for the published price.
	StatusShipped Status = "shipped"
	// StatusRefunded is the terminal failure state: the deadline
	// elapsed without a full pass set; the held amount was released
	// back to the wallet and the user pays nothing.
	StatusRefunded Status = "refunded"
	// StatusCancelled is the terminal user-driven state: the buyer
	// cancelled before either deadline or full pass set; the hold is
	// released, no debit, audit row retained.
	StatusCancelled Status = "cancelled"
)

// ShipPass is the unit of outcome-based billing. The TenantID column
// is the wallet owner the hold lives against (matches User.OrgID when
// the user belongs to an org, User.ID otherwise — same convention as
// wallet.Wallet.TenantID).
type ShipPass struct {
	ID          string
	TenantID    string
	ProjectID   string
	TierKey     string
	PriceUSD    decimal.Decimal
	Status      Status
	DeadlineAt  time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	SettledAt   *time.Time
	HoldOpKey   string // wallet idempotency key for the Hold
	DebitOpKey  string // wallet idempotency key for the Debit (set when shipping)
	RefundOpKey string // wallet idempotency key for the Release (set when refunding)
}

// GateProgress is the per-required-gate observation snapshot. The
// Settler stores one row per (pass, gate) every time it ingests a
// verdict; the latest row per pair drives the decision to ship or
// keep waiting. Storing every observation (not just the latest) is
// deliberate — it powers the "almost shipped" cohort the Feedback
// Brain mines.
type GateProgress struct {
	ID         string
	ShipPassID string
	Gate       domain.GateName
	Passed     bool
	Reason     string
	ObservedAt time.Time
}

// Quote is the resolver-facing preview of what a tenant would pay if
// they purchased the tier on this project right now. The frontend
// renders this before the buyer commits so they see the exact dollar
// figure and the exact gate scope they are buying against.
type Quote struct {
	TierKey         string
	PriceUSD        decimal.Decimal
	RequiredGates   []domain.GateName
	DeadlineDays    int
	WalletShortfall decimal.Decimal // zero when the wallet covers the price; positive amount the buyer must top up otherwise
}

// ErrInvalidTier is returned by Purchase when the tier key does not
// exist in the static catalogue (tiers.go). Bare error so resolvers
// can errors.Is it.
var ErrInvalidTier = errors.New("shippass: unknown tier")

// ErrPassNotFound is returned by lookups when the row is missing OR
// the caller's tenant does not own it. We deliberately collapse "not
// found" and "not yours" so probing the existence of another tenant's
// pass leaks no information.
var ErrPassNotFound = errors.New("shippass: not found")

// ErrPassNotActive is returned by mutation paths (Cancel, Settle)
// when the pass is already in a terminal state.
var ErrPassNotActive = errors.New("shippass: pass not active")

// ErrInsufficientWallet is returned by Purchase when the wallet
// available balance is below the tier price. The resolver translates
// this to a 402-style payload pointing at the top-up URL.
var ErrInsufficientWallet = errors.New("shippass: insufficient wallet balance")
