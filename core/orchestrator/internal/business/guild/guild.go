// Package guild is the FinisherGuild monetization layer.
//
// Two-sided marketplace. The supply side is human "finishers" (vetted
// developers) who bid on AI-failed gates the orchestrator could not
// close on its own; the demand side is requestors whose project hit
// the gate floor and chose to crowd-source the close. The second
// surface is finisher-grade "templates" (starter kits with auth +
// payments + deploy pre-wired) that template authors publish for a
// rev-share on every install.
//
// Economics (rake defaults — overridable by env in future):
//
//   - Guild work: Ironflyer keeps 20% of every accepted task amount;
//     the finisher takes 80%.
//   - Template installs: Ironflyer keeps 15% of every install price;
//     the template author takes 85%.
//
// Boundaries with neighbours:
//
//   - `wallet` is the funding source. Every task and install Holds in
//     the requestor's wallet before any work / artifact lands, then
//     Debits on accept / install. Hold/Release/Debit are the only
//     four wallet ops this package performs.
//   - `blueprints` is the in-repo starter registry. `guild.Template`
//     EXTENDS that concept for community-authored kits — same gate-
//     contract idea (a Template advertises the `GatesPassed` it is
//     CI-verified to satisfy), but rev-share-able and author-attributed.
//     The two registries stay separate so the in-repo built-ins keep
//     their compile-time guarantees.
//   - `provisioning` is the future home for the real Stripe Connect
//     payouts. `guild.payOutFinisher` is a clearly-named stub a
//     downstream agent will swap for a ProvisioningVault transfer.
//
// Hard rules this package upholds:
//
//   - All money is decimal.Decimal USD; no floats.
//   - Every state transition emits a learning.OutcomeEvent via
//     learning.Publish so the Feedback Brain can mine guild patterns
//     (which finishers close which gate types; which templates pay
//     back their install).
//   - Every operation that touches a Task / Template / Bid checks
//     ownership before mutating — finishers cannot withdraw bids they
//     did not place; requestors cannot accept bids on projects they
//     do not own; authors cannot edit templates they did not publish.
//   - Idempotency mirrors the wallet pattern: `accept_bid:<bidID>`
//     and `install_template:<installID>` op keys land in the
//     `guild_operations` table so Temporal retries do not double-
//     debit the wallet or double-payout a finisher.
package guild

import (
	"time"

	"github.com/shopspring/decimal"
)

// Status string constants. Mirror the CHECK constraints in
// migrations/000XX_guild.sql; new values land in both places.
const (
	TaskStatusOpen       = "open"
	TaskStatusBidding    = "bidding"
	TaskStatusInProgress = "in-progress"
	TaskStatusReview     = "review"
	TaskStatusAccepted   = "accepted"
	TaskStatusRejected   = "rejected"
	TaskStatusExpired    = "expired"

	BidStatusOpen      = "open"
	BidStatusWon       = "won"
	BidStatusLost      = "lost"
	BidStatusWithdrawn = "withdrawn"
)

// Rake fractions. Stored as decimal so all math stays exact.
// PlatformTaskCutPct = 0.20, PlatformTemplateCutPct = 0.15.
var (
	platformTaskCutPct     = decimal.NewFromFloat(0.20)
	platformTemplateCutPct = decimal.NewFromFloat(0.15)
)

// PlatformTaskCutPct returns Ironflyer's rake fraction on accepted
// guild tasks. Exposed for the Vault dashboard / forecast resolvers.
func PlatformTaskCutPct() decimal.Decimal { return platformTaskCutPct }

// PlatformTemplateCutPct returns Ironflyer's rake fraction on template
// installs.
func PlatformTemplateCutPct() decimal.Decimal { return platformTemplateCutPct }

// FinisherProfile is one human finisher's public-facing profile. The
// UserID links back to the orchestrator's User row — the same person
// can be a requestor on one project AND a finisher on someone else's,
// so we model the finisher identity as additive metadata rather than
// a User subtype. Verified is set by Ironflyer ops after the manual
// vetting flow (out of scope for this package — flipped via a
// future admin mutation).
type FinisherProfile struct {
	ID                 string
	UserID             string
	DisplayName        string
	Skills             []string
	HourlyRateUSD      decimal.Decimal
	CompletedTaskCount int
	Rating             decimal.Decimal
	Verified           bool
	CreatedAt          time.Time
}

// GuildTask is one piece of crowd-sourced work. ProjectID + TenantID
// scope ownership: only the project owner can create a task, only the
// project owner can accept a bid, and only the assigned finisher can
// mark the task ready for review.
//
// PriceUSDFloor is the requestor's CEILING — bids must come in AT OR
// BELOW this number, which is why we Hold the floor at task create
// time (so a winning bid can never blow the requestor's budget).
//
// GateFailureID links the task back to the gate verdict that
// triggered the router. May be empty for tasks the requestor opens by
// hand from the cockpit (no auto-router involvement).
type GuildTask struct {
	ID            string
	ProjectID     string
	TenantID      string
	GateFailureID string
	Title         string
	Description   string
	PriceUSDFloor decimal.Decimal
	SLAHours      int
	Status        string
	AssignedTo    *string
	CreatedAt     time.Time
	AcceptedAt    *time.Time
}

// Bid is one finisher's offer on a task. PriceUSD is the bid amount
// (must be <= task.PriceUSDFloor — enforced in PlaceBid). EstimatedHours
// is finisher-supplied effort for the requestor's ranking; Note is free
// text the requestor reads before accepting.
type Bid struct {
	ID             string
	TaskID         string
	FinisherID     string
	PriceUSD       decimal.Decimal
	EstimatedHours int
	Note           string
	Status         string
	CreatedAt      time.Time
}

// Template is one community-authored starter kit. PriceUSD is a one-
// time install fee; the author keeps 85% via the rev-share calculation
// in templates.go. GatesPassed is the list of finisher gates the
// Ironflyer CI verified this template satisfies — surfaced to the
// requestor so they can pick a template with the right gate coverage
// for their need.
//
// Slug is the user-facing URL fragment (kebab-case); the registry
// guards uniqueness so two authors cannot collide on the same slug.
type Template struct {
	ID           string
	AuthorUserID string
	Slug         string
	Name         string
	Description  string
	PriceUSD     decimal.Decimal
	GatesPassed  []string
	InstallCount int
	Verified     bool
	CreatedAt    time.Time
}

// Install is the per-install rev-share record. AuthorPayoutUSD +
// PlatformCutUSD MUST sum to AmountUSD — invariant enforced by
// templates.go::computeInstallSplit.
type Install struct {
	ID              string
	TemplateID      string
	ProjectID       string
	TenantID        string
	AmountUSD       decimal.Decimal
	AuthorPayoutUSD decimal.Decimal
	PlatformCutUSD  decimal.Decimal
	InstalledAt     time.Time
}

// Payout is one queued cash-out to a finisher after a task accept.
// PlatformCutUSD + FinisherCutUSD MUST sum to AmountUSD. Status
// transitions: pending -> paid | failed. The real Stripe Connect
// transfer lands the rail-side transfer id in ExternalRef when the
// PayoutTransferer is wired (see payouts.go).
type Payout struct {
	ID             string
	TaskID         string
	FinisherID     string
	AmountUSD      decimal.Decimal
	FinisherCutUSD decimal.Decimal
	PlatformCutUSD decimal.Decimal
	Status         string
	ExternalRef    string
	CreatedAt      time.Time
	CompletedAt    *time.Time
}
