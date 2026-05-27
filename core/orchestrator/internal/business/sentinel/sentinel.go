// Package sentinel implements Budget Sentinel — the user-facing,
// predictive complement to ProfitGuard.
//
// ProfitGuard is the per-call runtime gate: it decides whether the
// single next expensive call clears the margin / stop-loss / risk
// thresholds. Sentinel is the *project-wide forecast layer*: at the
// current burn rate, when will this project hit its cap, how does
// the trajectory compare to similar past projects, and what is the
// cheapest path to completion if the buyer wants to keep the cap?
//
// Why a separate package.
//
//   - ProfitGuard runs on every step and MUST stay pure (no I/O,
//     trivially reviewable). Sentinel runs on a slower cadence,
//     reads aggregated ledger state, and is allowed to talk to the
//     learning store for actuarial baselines.
//
//   - The Insured Ship SKU (a flat-fee cap with a "refund-if-
//     exceeded" guarantee) needs an actuarial model that combines
//     historical burn rate per project archetype with the current
//     trajectory. That logic does not belong in ProfitGuard.
//
//   - The dashboard panel reads from one Service surface — clients
//     never have to reconcile ProfitGuard verdicts with their own
//     extrapolation.
//
// Hard contracts:
//   - Owner check every read/write: callers pass a tenant; the
//     Service compares it to the project's owner.
//   - All money is decimal.Decimal USD.
//   - Forecasts NEVER mutate the wallet — they are pure projections.
//     The Insured Ship SKU mutates the wallet via the existing
//     wallet.IdempotentService surface.
package sentinel

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

// WarningLevel is the closed taxonomy of trajectory warnings the
// dashboard renders. The wire values are stable — they land in the
// notifications table and in the GraphQL response.
type WarningLevel string

const (
	// WarnGreen — burn rate is below 60% of cap on extrapolated ETA.
	WarnGreen WarningLevel = "green"
	// WarnYellow — between 60% and 80%; the buyer should consider a
	// reroute but no action is forced.
	WarnYellow WarningLevel = "yellow"
	// WarnOrange — between 80% and 95%; Sentinel surfaces a hard CTA
	// to reroute or top up.
	WarnOrange WarningLevel = "orange"
	// WarnRed — projection crosses 95% of cap; Sentinel auto-throttles
	// premium model usage on the next admit (via ProfitGuard policy
	// hint) and pages the buyer.
	WarnRed WarningLevel = "red"
)

// RerouteKind is the catalogue of cheaper-completion paths the
// suggestion engine can offer the buyer.
type RerouteKind string

const (
	// RerouteCheaperModel — switch the active model tier down one
	// rung (Opus → Sonnet, Sonnet → Haiku).
	RerouteCheaperModel RerouteKind = "cheaper_model"
	// RerouteTemplateSwap — replace a from-scratch build with a
	// known-shipping template that covers the project intent.
	RerouteTemplateSwap RerouteKind = "template_swap"
	// RerouteSmallerContext — drop reference attachments below a
	// threshold so the per-call prompt cost contracts.
	RerouteSmallerContext RerouteKind = "smaller_context"
	// RerouteSkipMobile — defer mobile build / Mac pool until the
	// web surface is shipped; halves cost on multi-surface projects.
	RerouteSkipMobile RerouteKind = "skip_mobile"
)

// Forecast is the dashboard-facing projection. ETACompletionAt is
// the extrapolated finish time at the current burn rate; HardCap is
// the policy ceiling Sentinel is comparing against. Level drives the
// chip color in the UI.
type Forecast struct {
	ProjectID                string
	TenantID                 string
	SpentUSD                 decimal.Decimal
	HardCapUSD               decimal.Decimal
	BurnRatePerHourUSD       decimal.Decimal
	ExtrapolatedTotalUSD     decimal.Decimal
	ETACompletionAt          time.Time
	CapBreachAt              *time.Time // non-nil when the trajectory crosses cap before completion
	Level                    WarningLevel
	RemainingHeadroomUSD     decimal.Decimal
	ProjectionConfidenceFrac float64 // [0, 1]; 0 = no history, 1 = many similar past completions
	ComputedAt               time.Time
}

// Reroute is one suggestion the engine offers the buyer. SavingsUSD
// is the extrapolated dollar saving versus the current trajectory;
// SavingsConfidence reflects the actuarial certainty (similar past
// projects that took this reroute and shipped). The frontend
// dashboard renders rows top-to-bottom by SavingsUSD.
type Reroute struct {
	Kind              RerouteKind
	Label             string
	Description       string
	SavingsUSD        decimal.Decimal
	SavingsConfidence float64
	Reversible        bool
}

// InsurancePolicy is the Insured Ship SKU shape: a flat fee paid up
// front buys the buyer a hard cap with a refund-if-exceeded
// guarantee. The actuarial calc lives in insurance.go; this struct
// is the persisted contract.
type InsurancePolicy struct {
	ID                  string
	TenantID            string
	ProjectID           string
	HardCapUSD          decimal.Decimal
	PremiumUSD          decimal.Decimal
	CoverageWindowHours int
	Status              string // "active" | "paid_out" | "expired" | "cancelled"
	CreatedAt           time.Time
	UpdatedAt           time.Time
	ExpiresAt           time.Time
	PremiumOpKey        string
	PayoutOpKey         string
}

// ErrNoProject is returned when the project id is missing or not
// owned by the caller. We deliberately collapse the two failure
// modes so probing leaks no information.
var ErrNoProject = errors.New("sentinel: project not found")

// ErrInvalidPolicy is returned when the requested insurance policy
// parameters cannot be priced (negative cap, zero coverage window).
var ErrInvalidPolicy = errors.New("sentinel: invalid insurance policy")

// ErrPolicyAlreadyActive is returned when a project already has an
// active insurance policy.
var ErrPolicyAlreadyActive = errors.New("sentinel: policy already active")

// ErrPolicyNotFound is returned by lookups when the policy is missing
// or not owned by the caller.
var ErrPolicyNotFound = errors.New("sentinel: policy not found")
