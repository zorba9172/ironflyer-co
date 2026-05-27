package wallet

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
)

// Provider names for wallet top-up checkout. Keep them lowercase and
// stable — webhooks, ledger rows, and frontend toggles key off these
// strings.
const (
	ProviderStripe = "stripe"
	ProviderPaddle = "paddle"
)

// CheckoutSession is the provider-neutral result of a top-up checkout
// creation. The browser navigates to URL; SessionID is opaque and
// exists so the dashboard can correlate a pending row with the
// in-flight checkout. Provider identifies which PSP minted the
// session.
type CheckoutSession struct {
	URL       string
	SessionID string
	Provider  string
}

// VerifyStatus is the provider-neutral terminal state of a checkout
// session as reported by the vendor's read API. Reconciler uses it
// to recover from missed webhooks: paid sessions become wallet
// credits; expired / failed sessions move out of 'pending'.
type VerifyStatus string

const (
	VerifyPaid    VerifyStatus = "paid"
	VerifyOpen    VerifyStatus = "open"
	VerifyExpired VerifyStatus = "expired"
	VerifyFailed  VerifyStatus = "failed"
)

// VerifyResult is the typed shape Reconciler consumes. Amount is the
// provider-reported USD amount on `paid` sessions; zero otherwise.
type VerifyResult struct {
	Status VerifyStatus
	Amount decimal.Decimal
}

// Topper is the provider-neutral contract for a wallet top-up
// integration. Implementations wrap the vendor checkout API + webhook
// signature verification, and credit the backing Service when payment
// confirms.
//
// CreateCheckoutSession stages a pending wallet_topups row before
// returning the URL; HandleWebhook flips it to succeeded on payment.
// Both calls are safe to retry — idempotency is anchored on the
// returned SessionID (Stripe cs_*, Paddle txn_*).
type Topper interface {
	// Name is the stable provider identifier used in ledger rows,
	// webhook routes, and the frontend provider toggle. Lowercase.
	Name() string
	// Enabled reports whether this Topper has the credentials it
	// needs. Disabled toppers stay in the registry but never reach
	// the user.
	Enabled() bool
	// Label is the human-readable name shown on the wallet top-up
	// UI. Distinct from Name so we can render "Card (Stripe)" vs.
	// "Card (Paddle, MoR)" without leaking the provider id.
	Label() string
	// CreateCheckoutSession provisions a one-shot checkout for the
	// given tenant + amount and stages a pending wallet_topups row.
	CreateCheckoutSession(ctx context.Context, tenant string, amountUSD decimal.Decimal) (CheckoutSession, error)
	// HandleWebhook verifies the inbound vendor webhook and credits
	// the wallet via Service.TopUp. Idempotent against retries.
	HandleWebhook(ctx context.Context, rawBody []byte, signatureHeader string) error
	// VerifySession queries the vendor's read API for the terminal
	// state of a checkout session. Used by the reconciliation cron
	// to recover when a webhook fails to land (rare, but happens —
	// vendor-side queue lag, our 503 during a deploy, network split).
	VerifySession(ctx context.Context, sessionID string) (VerifyResult, error)
}

// ErrTopperDisabled is returned by registry lookups when no enabled
// topper matches the request.
var ErrTopperDisabled = errors.New("wallet: no enabled top-up provider")

// TopperRegistry holds the set of Toppers an orchestrator boot has
// configured, in priority order. The primary topper is the default
// rendered on /wallet/topup; alternatives are surfaced as "card
// declined? try alternative checkout →" links so a single PSP outage
// never stalls revenue. The registry intentionally does NOT do auto
// server-side failover between providers — the reconciliation cost
// of late-arriving webhooks from two providers on the same intent
// is higher than the value of the convenience.
type TopperRegistry struct {
	primary      Topper
	alternatives []Topper
}

// NewTopperRegistry builds a registry. Nil + disabled toppers are
// tolerated — Active() filters them out — so wireup code can pass in
// every potential Topper unconditionally.
func NewTopperRegistry(primary Topper, alternatives ...Topper) *TopperRegistry {
	return &TopperRegistry{primary: primary, alternatives: alternatives}
}

// Primary returns the preferred enabled Topper. When the configured
// primary is disabled, falls through to the first enabled alternative
// so a Stripe-out-of-action deploy still surfaces a working CTA.
// Returns nil when nothing is enabled.
func (r *TopperRegistry) Primary() Topper {
	if r == nil {
		return nil
	}
	if r.primary != nil && r.primary.Enabled() {
		return r.primary
	}
	for _, t := range r.alternatives {
		if t != nil && t.Enabled() {
			return t
		}
	}
	return nil
}

// Active returns every enabled Topper, primary first. Resolvers fan
// out provider chips to the frontend from this list. The slice is
// fresh on every call so callers may mutate it.
func (r *TopperRegistry) Active() []Topper {
	if r == nil {
		return nil
	}
	seen := map[string]bool{}
	out := make([]Topper, 0, 1+len(r.alternatives))
	if r.primary != nil && r.primary.Enabled() {
		out = append(out, r.primary)
		seen[r.primary.Name()] = true
	}
	for _, t := range r.alternatives {
		if t == nil || !t.Enabled() || seen[t.Name()] {
			continue
		}
		out = append(out, t)
		seen[t.Name()] = true
	}
	return out
}

// ByName returns the enabled Topper matching name (case-insensitive).
// Empty name returns the primary. Returns ErrTopperDisabled when the
// requested provider is unknown or disabled.
func (r *TopperRegistry) ByName(name string) (Topper, error) {
	if r == nil {
		return nil, ErrTopperDisabled
	}
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" {
		if p := r.Primary(); p != nil {
			return p, nil
		}
		return nil, ErrTopperDisabled
	}
	for _, t := range r.Active() {
		if t.Name() == n {
			return t, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrTopperDisabled, n)
}

// Enabled is true when at least one Topper is configured + enabled.
// Resolvers use this for the gqlNotConfigured guard so the mutation
// returns a clean error in dev environments without any PSP creds.
func (r *TopperRegistry) Enabled() bool {
	return r != nil && r.Primary() != nil
}

// FindForSessionID returns the Topper that minted the given session
// id, based on the vendor-specific prefix (Stripe cs_*, Paddle
// txn_*). Used by the reconciliation cron to route refunds /
// chargebacks back to the right vendor without persisting provider
// per row historically. Returns ErrTopperDisabled when no Topper
// matches.
func (r *TopperRegistry) FindForSessionID(sessionID string) (Topper, error) {
	if r == nil {
		return nil, ErrTopperDisabled
	}
	switch {
	case strings.HasPrefix(sessionID, "cs_"):
		return r.ByName(ProviderStripe)
	case strings.HasPrefix(sessionID, "txn_"):
		return r.ByName(ProviderPaddle)
	default:
		return nil, fmt.Errorf("%w: unrecognised session id prefix", ErrTopperDisabled)
	}
}
