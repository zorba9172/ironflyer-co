package wowloop

import "github.com/shopspring/decimal"

// nextActionInput is the compacted view of the bundle's already-
// assembled sections that the heuristic needs. Pulled out so the
// heuristic stays a pure function of its inputs and can be reviewed
// in isolation.
type nextActionInput struct {
	Status            string
	ExecutionID       string
	PreviewURL        string
	HasPreview        bool
	HasProduction     bool
	HasSecurityIssue  bool
	SecurityBlocks    bool
	WalletBalanceUSD  decimal.Decimal
	WalletHoldsActive bool
	PatchCount        int
}

// lowBalanceThreshold is the wallet floor below which the heuristic
// promotes the top_up action (when no other action takes priority).
// Five dollars is the V22 launch floor — enough to cover one small
// premium reasoning call so the user is not blocked on their next try.
var lowBalanceThreshold = decimal.NewFromFloat(5.0)

// decideNextAction is the heuristic table. Ordered from highest
// priority (a blocking failure) to lowest (the catch-all review
// nudge).
//
// Heuristic table:
//
//   priority | trigger                                  | kind
//   ---------+------------------------------------------+----------------------
//   1        | status=failed AND security findings      | fix_security_finding
//   2        | security blocks deploy                   | fix_security_finding
//   3        | succeeded AND no preview                 | deploy
//   4        | succeeded AND has preview AND no prod    | share_preview
//   5        | holds released AND balance < threshold   | top_up
//   6        | otherwise                                | review_patch
func decideNextAction(in nextActionInput) NextAction {
	execPath := ""
	if in.ExecutionID != "" {
		execPath = "/app/executions/" + in.ExecutionID
	}
	switch {
	case in.Status == "failed" && in.HasSecurityIssue:
		return NextAction{
			Kind:   "fix_security_finding",
			Title:  "Fix the blocking security finding",
			Reason: "Execution failed and the security gate raised a finding that needs a patch before the next run.",
			CTA:    ctaOr(execPath+"#security", "/app/executions"),
		}
	case in.SecurityBlocks:
		return NextAction{
			Kind:   "fix_security_finding",
			Title:  "Resolve the security finding blocking deploy",
			Reason: "The security gate flagged a high/critical finding that is keeping the deploy gate red.",
			CTA:    ctaOr(execPath+"#security", "/app/executions"),
		}
	case in.Status == "succeeded" && !in.HasPreview:
		return NextAction{
			Kind:   "deploy",
			Title:  "Ship the preview deploy",
			Reason: "The execution finished cleanly but no preview is live yet — promote it so you can share it.",
			CTA:    ctaOr(execPath+"#deploy", "/app/executions"),
		}
	case in.Status == "succeeded" && in.HasPreview && !in.HasProduction:
		return NextAction{
			Kind:   "share_preview",
			Title:  "Share the preview URL",
			Reason: "Preview is live. Share the link with reviewers or promote it to production.",
			CTA:    ctaOr(in.PreviewURL, execPath),
		}
	case !in.WalletHoldsActive && in.WalletBalanceUSD.LessThan(lowBalanceThreshold):
		return NextAction{
			Kind:   "top_up",
			Title:  "Top up your wallet",
			Reason: "The wallet hold released but the remaining balance is below the safe floor for the next execution.",
			CTA:    "/app/wallet",
		}
	default:
		return NextAction{
			Kind:   "review_patch",
			Title:  "Review the patches that landed",
			Reason: "Walk through the applied patches to confirm intent before kicking off the next iteration.",
			CTA:    ctaOr(execPath+"#patches", "/app/executions"),
		}
	}
}

// ctaOr returns primary when non-empty, else fallback. Keeps the
// heuristic table readable while letting each branch degrade
// gracefully when the execution id is missing.
func ctaOr(primary, fallback string) string {
	if primary != "" && primary != "#security" && primary != "#deploy" && primary != "#patches" {
		return primary
	}
	return fallback
}
