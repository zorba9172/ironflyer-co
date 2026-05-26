package wireup

import (
	"context"
	"fmt"

	"github.com/shopspring/decimal"

	"ironflyer/apps/orchestrator/internal/domain"
	"ironflyer/apps/orchestrator/internal/ledger"
	"ironflyer/apps/orchestrator/internal/profitguard"
)

// MobileReservation is the receipt returned by ReserveMobileBuild. It
// captures the worst-case ledger entry the caller is expected to post
// once the build completes (or partially complete on a failure path),
// plus the matching ProfitGuard verdict so the runtime can audit why
// the build was admitted. Callers MUST treat ReservedUSD as a hold —
// the actual cost lands via ledger.RecordMobileBuildMinutes /
// ledger.RecordEASBuild as the build progresses, and any unused
// portion is released through EntryCreditRelease on commit.
type MobileReservation struct {
	// Category names the dominant cost leg of the reservation —
	// CategoryMobileBuildMin for sandbox-based builds, CategoryEASBuildCredit
	// for EAS-managed builds, CategoryMacWorkspaceMin for native iOS.
	Category ledger.Category
	// EstimatedMinutes is the conservative estimate ProfitGuard used
	// when reserving against the wallet. Mostly informational; the
	// runtime should re-record actuals after the build.
	EstimatedMinutes float64
	// ReservedUSD is the dollar hold against the wallet. Equal to
	// EstimatedMinutes * ledger.UnitPriceUSD(Category) for the
	// minute-priced categories, or a flat credit for EAS builds.
	ReservedUSD decimal.Decimal
	// Decision is the underlying ProfitGuard verdict — kept on the
	// receipt so the caller can record the audit trail.
	Decision profitguard.Decision
}

// estimateMobileBuild returns the (category, minutes) tuple
// ReserveMobileBuild reserves against. The numbers are conservative
// upper bounds — a release Android build with signing typically lands
// in the 4-8 minute range; iOS-on-Mac runs 8-15; EAS adds a flat
// credit plus a few minutes of supervision. Calibrate against real
// invoice data once we have a month of traffic.
func estimateMobileBuild(kind domain.MobileKind, target domain.MobileTarget) (ledger.Category, float64) {
	switch target {
	case domain.MobileTargetIOS:
		// Native iOS goes through the Mac pool; Expo iOS goes through
		// EAS. Pick the category that dominates the bill.
		if kind == domain.MobileKindExpo {
			return ledger.CategoryEASBuildCredit, 1
		}
		return ledger.CategoryMacWorkspaceMin, 15
	case domain.MobileTargetAndroid:
		if kind == domain.MobileKindExpo {
			return ledger.CategoryEASBuildCredit, 1
		}
		// Linux-sandbox Android build: 4-8 min build + 5 min buffer.
		return ledger.CategoryMobileBuildMin, 13
	}
	// Unknown target — assume Linux sandbox with a generous buffer.
	return ledger.CategoryMobileBuildMin, 13
}

// ReserveMobileBuild reserves wallet funds for a mobile build before
// any expensive runtime call starts. The estimate is conservative —
// a release Android build with signing typically lands in the 4-8
// minute range, plus 5 minutes of buffer; a release iOS build via
// EAS bills as one credit, plus expected runtime minutes on our side.
//
// Returns a MobileReservation handle when the guard admits the build,
// or an error when ProfitGuard halts it (Stop / PauseForBudget /
// KillBranch). Caller is responsible for releasing the reservation on
// completion by posting the actual cost via ledger.RecordMobile* and
// then a matching EntryCreditRelease for the unused portion.
func ReserveMobileBuild(
	ctx context.Context,
	guard profitguard.Guard,
	state profitguard.ExecState,
	kind domain.MobileKind,
	target domain.MobileTarget,
) (MobileReservation, error) {
	if guard == nil {
		return MobileReservation{}, fmt.Errorf("profitguard: nil guard")
	}
	category, minutes := estimateMobileBuild(kind, target)
	var reserved decimal.Decimal
	if category == ledger.CategoryEASBuildCredit {
		reserved = ledger.PriceEASBuildCreditUSD
	} else {
		reserved = decimal.NewFromFloat(minutes).Mul(ledger.UnitPriceUSD(category))
	}

	// Fold the reservation into the snapshot so the guard arithmetic
	// sees this build as the next step. We do NOT mutate the caller's
	// state — copy first.
	snap := state
	snap.EstimatedNextStepCostUSD = snap.EstimatedNextStepCostUSD.Add(reserved)
	snap.EstimatedPlatformCostUSD = snap.EstimatedPlatformCostUSD.Add(reserved)

	dec, err := guard.Decide(ctx, profitguard.BeforeMobileBuild, snap)
	if err != nil {
		return MobileReservation{}, fmt.Errorf("profitguard: decide: %w", err)
	}
	_ = guard.Record(ctx, state.ExecutionID, profitguard.BeforeMobileBuild, dec, snap)

	switch dec.Action {
	case profitguard.Continue, profitguard.Degrade,
		profitguard.SwitchProvider, profitguard.ReuseBlueprint,
		profitguard.ReuseRepair:
		// Admitted (possibly with a downgrade hint the caller honours).
		return MobileReservation{
			Category:         category,
			EstimatedMinutes: minutes,
			ReservedUSD:      reserved,
			Decision:         dec,
		}, nil
	default:
		// Stop / PauseForBudget / KillBranch — refuse the build.
		return MobileReservation{}, fmt.Errorf("profitguard: %s: %s", dec.Action, dec.Reason)
	}
}
