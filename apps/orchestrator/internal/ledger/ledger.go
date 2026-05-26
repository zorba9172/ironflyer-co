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

	// Mobile-specific cost categories. ProfitGuard reserves against these
	// before allocating the underlying resource so the wallet contract is
	// preserved even for long-running native builds.
	//
	// NOTE: the Postgres CHECK constraint in migrations/00025_ledger.sql
	// will reject these values until a follow-up migration extends the
	// allow-list. The in-memory backend accepts them immediately so
	// dev/local mode + simulators can exercise mobile cost flows today.

	// EntryMobileBuildMin meters minutes spent producing a mobile build
	// artifact (APK/AAB/IPA/.app) inside a Linux sandbox. Roughly mirrors
	// the sandbox-minute cost but tagged separately so cost dashboards
	// can show "Android build" vs "Web build". Debit.
	EntryMobileBuildMin EntryType = "mobile_build_min"

	// EntryEmulatorMin meters minutes the Android emulator has been
	// running inside a sandbox. Higher unit cost than EntryMobileBuildMin
	// because the emulator pins a vCPU + KVM passthrough for its entire
	// lifetime. Debit.
	EntryEmulatorMin EntryType = "emulator_min"

	// EntryMacWorkspaceMin meters minutes consumed on a Mac pool host
	// (Scaleway / MacStadium / AWS mac2.metal). This is the iOS Pro tier
	// cost floor — every Apple-licensed second a workspace holds a Mac
	// mini is billed here regardless of CPU utilisation, mirroring
	// Apple's 24-hour minimum lease. Debit.
	EntryMacWorkspaceMin EntryType = "mac_workspace_min"

	// EntryEASBuildCredit meters one Expo Application Services build
	// credit consumed via `eas build`. EAS bills in build minutes on its
	// own side; we model each successful build as a fixed-cost credit
	// here for simplicity, then reconcile against the EAS invoice
	// nightly. Debit.
	EntryEASBuildCredit EntryType = "eas_build_credit"

	// EntryAppetizeMin meters minutes streamed via Appetize.io's iOS
	// simulator-in-browser fallback (used when the project is iOS-bound
	// but the user is on the free tier — no dedicated Mac workspace).
	// Debit.
	EntryAppetizeMin EntryType = "appetize_min"

	// EntryDeviceCloudMin meters minutes consumed against a Pro-tier
	// device-cloud provider (BrowserStack App Live today, AWS Device
	// Farm tomorrow). Unit cost is the platform cost basis — we sell
	// at a higher per-minute rate; the difference is the platform
	// margin recorded at execution commit. Debit.
	EntryDeviceCloudMin EntryType = "device_cloud_min"

	// EntryFreeToPaidConversion records the moment a free-tier user
	// upgrades to a paid plan. Amount = the first-month subscription
	// price (the LTV starting point); Metadata carries `previous_tier`,
	// `new_tier`, and `acquisition_cost_usd` when known. Credit.
	//
	// Why a dedicated ledger entry rather than a vault-only event:
	// the wallet ledger is the auditable money trail. Without a typed
	// conversion entry the dashboards can compute MRR but cannot
	// compute the most important growth metric — Free→Paid conversion
	// rate — without scraping subscription history. A typed entry
	// lets `SELECT count(*) WHERE entry_type='free_to_paid_conversion'
	// AND created_at > now()-interval '30 days'` answer it in O(1).
	//
	// NOTE: same Postgres CHECK constraint caveat as the mobile
	// entries above — the in-memory ledger accepts this today; a
	// follow-up migration extends the allow-list.
	EntryFreeToPaidConversion EntryType = "free_to_paid_conversion"
)

// Category is an alias for EntryType used in mobile-cost call sites
// where "category" reads more naturally than "entry type". The
// underlying values are identical — Category and EntryType compare
// equal — so existing callers do not need to change.
type Category = EntryType

// Mobile cost-category aliases. The Category* names mirror the
// V22 mobile cost taxonomy described in the deep atomic plan and
// keep the call site at, e.g., ledger.CategoryMobileBuildMin readable
// without forcing callers to spell out the longer Entry* form.
const (
	CategoryMobileBuildMin   Category = EntryMobileBuildMin
	CategoryEmulatorMin      Category = EntryEmulatorMin
	CategoryMacWorkspaceMin  Category = EntryMacWorkspaceMin
	CategoryEASBuildCredit   Category = EntryEASBuildCredit
	CategoryAppetizeMin      Category = EntryAppetizeMin
	CategoryDeviceCloudMin   Category = EntryDeviceCloudMin
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
	EntryMobileBuildMin,
	EntryEmulatorMin,
	EntryMacWorkspaceMin,
	EntryEASBuildCredit,
	EntryAppetizeMin,
	EntryDeviceCloudMin,
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
