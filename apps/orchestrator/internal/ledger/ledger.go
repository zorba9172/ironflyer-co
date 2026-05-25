// Package ledger is the append-only financial record of every dollar
// that moves through Ironflyer. Every wallet top-up, every reserved
// hold, every provider inference cost, every sandbox tick, every
// refund, and every platform margin entry lands here as an immutable
// row. The ledger is the proof surface for "did this execution
// actually run at a positive gross margin?" — V22 law 3.
//
// Append-only by contract: the Service interface only exposes Write
// + read operations. Refunds and credit releases are new entries,
// never UPDATEs. The Postgres schema (migrations/00025_ledger.sql)
// enforces the same shape at the storage layer with a CHECK on
// entry_type and direction.
package ledger

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// EntryType enumerates every legal kind of ledger row. The constants
// here must stay in lock-step with the CHECK constraint in
// migrations/00025_ledger.sql — a value defined in Go but not in the
// migration will fail INSERT at runtime, and vice versa.
type EntryType string

const (
	// EntryWalletTopup — money in from Stripe Checkout. Credit.
	EntryWalletTopup EntryType = "wallet_topup"
	// EntryCreditReservation — execution start hold. Debit (hold).
	EntryCreditReservation EntryType = "credit_reservation"
	// EntryProviderInferenceCost — provider token charge. Debit.
	EntryProviderInferenceCost EntryType = "provider_inference_cost"
	// EntrySandboxCost — per-tick sandbox compute charge. Debit.
	EntrySandboxCost EntryType = "sandbox_cost"
	// EntryStorageCost — artifact / workspace storage. Debit.
	EntryStorageCost EntryType = "storage_cost"
	// EntryDeploymentCost — Vercel / runtime deploy egress. Debit.
	EntryDeploymentCost EntryType = "deployment_cost"
	// EntryRefund — money returned to wallet. Credit.
	EntryRefund EntryType = "refund"
	// EntryCreditRelease — unused reservation given back on commit. Credit.
	EntryCreditRelease EntryType = "credit_release"
	// EntryPlatformMargin — platform margin booked at execution commit. Credit.
	EntryPlatformMargin EntryType = "platform_margin"
	// EntryPremiumReasoningCharge — premium model surcharge. Debit.
	EntryPremiumReasoningCharge EntryType = "premium_reasoning_charge"
)

// AllEntryTypes is the canonical set used by validation and by the
// SumByType helper when callers want a full tenant rollup.
var AllEntryTypes = []EntryType{
	EntryWalletTopup,
	EntryCreditReservation,
	EntryProviderInferenceCost,
	EntrySandboxCost,
	EntryStorageCost,
	EntryDeploymentCost,
	EntryRefund,
	EntryCreditRelease,
	EntryPlatformMargin,
	EntryPremiumReasoningCharge,
}

// IsValid reports whether t is one of the canonical entry types.
func (t EntryType) IsValid() bool {
	for _, k := range AllEntryTypes {
		if k == t {
			return true
		}
	}
	return false
}

// Direction is the sign on the ledger row. amount_usd is always > 0;
// direction is what determines whether the entry adds to or subtracts
// from the tenant's net position. We split sign from magnitude so
// reports can sum either side without worrying about negative-number
// arithmetic.
type Direction string

const (
	DebitDirection  Direction = "debit"
	CreditDirection Direction = "credit"
)

// IsValid reports whether d is one of the canonical directions.
func (d Direction) IsValid() bool {
	return d == DebitDirection || d == CreditDirection
}

// Entry is one immutable financial event. ID and CreatedAt are
// assigned by the Service if zero at Write time so callers don't have
// to wire a clock or a UUID generator.
//
// OpKey is the V22 idempotency handle. When set, Write deduplicates on
// the (op_key) unique index in ledger_entries — a retried call with
// the same OpKey returns the existing row instead of inserting a
// second one. Empty OpKey is legacy / no-dedupe semantics, preserved
// so existing callers keep working unchanged.
type Entry struct {
	ID             uuid.UUID              `json:"id"`
	TenantID       uuid.UUID              `json:"tenantID"`
	ExecutionID    *uuid.UUID             `json:"executionID,omitempty"`
	EntryType      EntryType              `json:"entryType"`
	Direction      Direction              `json:"direction"`
	AmountUSD      decimal.Decimal        `json:"amountUSD"`
	Provider       string                 `json:"provider,omitempty"`
	Billable       bool                   `json:"billable"`
	MarginRelevant bool                   `json:"marginRelevant"`
	Metadata       map[string]any         `json:"metadata"`
	OpKey          string                 `json:"opKey,omitempty"`
	CreatedAt      time.Time              `json:"createdAt"`
}

// Filter is the query envelope used by ListByTenant. Every field is
// optional; zero values mean "no constraint on this axis". The
// postgres implementation translates the struct into a parameterised
// WHERE clause, never string-interpolated SQL.
type Filter struct {
	Since       time.Time
	Until       time.Time
	EntryTypes  []EntryType
	ExecutionID *uuid.UUID
	Limit       int
	Offset      int
}
